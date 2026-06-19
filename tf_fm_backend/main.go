//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

// Implements the http Backend state for TF, and allows the user to use FM
// MongoDB as the backend. This will allow the customer to use mongoDB as a shared
// state for all the users of FM Terraform Provider

// Runs in FM and is front-ended by HA Proxy. HA proxy is set to route all urls with
// /terraform-state to this service. Runs as a systemd service tf_backend

// Dependencies - This depends on MongoDB, and HA Proxy service.

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Each project has exactly one document in the "terraformBackendState" collection.
// The document stores both the terraform state and the current lock (if any).
// A unique index on "project" enforces one document per project.

const (
	defaultMongoURI   = "mongodb://localhost:27017/?replicaSet=fmreplica"
	defaultAuthSvcURL = "http://127.0.0.1:6687/authorize"
	databaseName      = "fmdb2"
	backendCollection = "terraformBackendState"

	// Body size limits — guard against DoS / zip-bomb attacks.
	// maxStateBodyBytes is set below MongoDB's 16 MB document limit to leave room for
	// BSON overhead and other fields.
	maxStateBodyBytes    int64 = 14 * 1024 * 1024  // 14 MB compressed state
	maxLockBodyBytes     int64 = 64 * 1024         // 64 KB lock/unlock payload
	maxDecompressedBytes int64 = 100 * 1024 * 1024 // 100 MB decompressed state
	maxAuthResponseBytes int64 = 1 * 1024 * 1024   // 1 MB auth service response
)

// validProjectName restricts project identifiers to safe characters and a bounded length.
var validProjectName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.$#-]{0,255}$`)

var errUnauthorizedUser = errors.New("unauthorized user")

type server struct {
	mongoClient        *mongo.Client
	authServiceBaseURL string
	httpClient         *http.Client
}

// Single document per project storing both state and lock.
type terraformProjectDocument struct {
	MongoID              bson.ObjectID `bson:"_id,omitempty"`
	Project              string        `bson:"project"`
	StateUsername        string        `bson:"state_username,omitempty"`
	Body                 bson.Binary   `bson:"body,omitempty"`
	CompressionAlgorithm string        `bson:"compression_algorithm,omitempty"`
	StateUpdateTime      time.Time     `bson:"state_update_time,omitempty"`
	LockID               string        `bson:"lock_id,omitempty"`
	LockUsername         string        `bson:"lock_username,omitempty"`
	LockTime             time.Time     `bson:"lock_time,omitempty"`
}

// Lock/Unlock request body sent by TF.
type terraformLockRequest struct {
	LockID string `json:"ID"`
}

type authorizationRequest struct {
	Token              string `json:"token"`
	RequiredPermission struct {
		Type   string `json:"type"`
		Action string `json:"action"`
	} `json:"required_permission"`
}

type authorizationResponse struct {
	Authorized bool   `json:"authorized"`
	Username   string `json:"username"`
	Error      string `json:"error,omitempty"`
}

// Validates if the owner of this token is authorized to do the operation by delegating
// authorization to the fm_auth_service.

func (appServer *server) validateAuthorizationToken(
	ctx context.Context,
	token string,
	requiredPermission [2]string,
) (string, error) {
	requestBody := authorizationRequest{Token: strings.TrimSpace(token)}
	requestBody.RequiredPermission.Type = strings.TrimSpace(requiredPermission[0])
	requestBody.RequiredPermission.Action = strings.TrimSpace(requiredPermission[1])

	if requestBody.Token == "" ||
		requestBody.RequiredPermission.Type == "" ||
		requestBody.RequiredPermission.Action == "" {
		return "", errUnauthorizedUser
	}

	marshaledRequest, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(
		authCtx,
		http.MethodPost,
		appServer.authServiceBaseURL,
		bytes.NewReader(marshaledRequest),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := appServer.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return "", errUnauthorizedUser
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authorization service returned status %d", response.StatusCode)
	}

	var authResponse authorizationResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, maxAuthResponseBytes)).Decode(&authResponse); err != nil {
		return "", err
	}

	if !authResponse.Authorized || strings.TrimSpace(authResponse.Username) == "" {
		return "", errUnauthorizedUser
	}

	return strings.TrimSpace(authResponse.Username), nil
}

// Currently in http remote backend, TF does not provide a way to pass the Bearer token.
// It supports only Basic authentication which will be a base64 encoded string of
// userid:password and the schem would be Basic

// Opentofu http backend supports the Bearer token directly. So for now support both schemes
// but simply allow the user to pass the token in the password field of the Basic scheme. The
// user id component of Basic scheme is simply ignored. So we will accept either Bearer scheme
// with the token directly, or Basic scheme with the token in the password field

func (appServer *server) requireBearerAuthorization(requiredPermission [2]string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authorizationHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if authorizationHeader == "" {
			ctx.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "missing Authorization header",
				},
			)
			return
		}

		scheme, token, found := strings.Cut(authorizationHeader, " ")
		token = strings.TrimSpace(token)
		if !found || token == "" {
			ctx.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "Authorization header must have none-zero token",
				},
			)
			return
		}

		if strings.EqualFold(scheme, "Basic") {
			decodedCredentials, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				ctx.AbortWithStatusJSON(
					http.StatusUnauthorized,
					gin.H{
						"error": "invalid Basic authorization token",
					},
				)
				return
			}

			_, token, found = strings.Cut(string(decodedCredentials), ":")
			token = strings.TrimSpace(token)
			if !found || token == "" {
				ctx.AbortWithStatusJSON(
					http.StatusUnauthorized,
					gin.H{
						"error": "Basic authorization must be username:password",
					},
				)
				return
			}
		} else if !strings.EqualFold(scheme, "Bearer") {
			ctx.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "Authorization header must use Bearer token",
				},
			)
			return
		}

		username, err := appServer.validateAuthorizationToken(
			ctx.Request.Context(),
			token,
			requiredPermission,
		)
		if err != nil {
			if errors.Is(err, errUnauthorizedUser) {
				ctx.AbortWithStatusJSON(
					http.StatusUnauthorized,
					gin.H{
						"error": "unauthorized user",
					},
				)
				return
			}

			ctx.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"error": "failed to validate authorization token",
				},
			)
			return
		}

		ctx.Set("username", username)

		ctx.Next()
	}
}

// validateProjectParam rejects requests whose :project value contains unsafe characters
// or exceeds a reasonable length, preventing injection and unbounded key sizes.
func validateProjectParam(ctx *gin.Context) {
	if !validProjectName.MatchString(ctx.Param("project")) {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid project name"})
		return
	}
	ctx.Next()
}

// List of URL that we provide for the http backend of TF. The project is an instance of
// infra that the customer wants to manage.
func setupRouter(appServer *server) *gin.Engine {
	// Use gin.New() instead of gin.Default() so the built-in logger does not write
	// full URLs (including ?ID= query parameters that contain the lock token) to logs.
	router := gin.New()

	// Recovery middleware: log the error message only — no stack trace that could
	// contain in-memory state data.
	router.Use(gin.CustomRecovery(func(ctx *gin.Context, recovered any) {
		log.Printf("panic recovered: %v", recovered)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}))

	// Minimal access log: method, path (no query string), status, latency.
	router.Use(func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()
		log.Printf("%s %s %d %s", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status(), time.Since(start))
	})

	defaultPermission := [2]string{"TRAFFIC_CONTROL", "write"}
	adminPermission := [2]string{"ALL", "ALL"}

	router.GET(
		"/terraform-state/:project",
		validateProjectParam,
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.getTerraformState,
	)
	router.POST(
		"/terraform-state/:project",
		validateProjectParam,
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.createTerraformState,
	)
	router.GET(
		"/terraform-state/:project/lock",
		validateProjectParam,
		appServer.requireBearerAuthorization(adminPermission),
		appServer.getTerraformStateLock,
	)
	router.DELETE(
		"/terraform-state/:project/lock",
		validateProjectParam,
		appServer.requireBearerAuthorization(adminPermission),
		appServer.deleteTerraformStateLock,
	)
	router.Handle(
		"LOCK",
		"/terraform-state/:project/lock",
		validateProjectParam,
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.lockTerraformState,
	)
	router.Handle(
		"UNLOCK",
		"/terraform-state/:project/lock",
		validateProjectParam,
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.unlockTerraformState,
	)

	return router
}

// Setup the MongoDB client, verify connectivity, and ensure the shared collection index.
func newServer(ctx context.Context) (*server, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(defaultMongoURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	appServer := &server{
		mongoClient:        client,
		authServiceBaseURL: defaultAuthSvcURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	if err := appServer.ensureBackendIndex(ctx); err != nil {
		return nil, fmt.Errorf("ensure backend index: %w", err)
	}

	return appServer, nil
}

func (appServer *server) close(ctx context.Context) error {
	return appServer.mongoClient.Disconnect(ctx)
}

// Create the unique index on (project) once at startup.
func (appServer *server) ensureBackendIndex(ctx context.Context) error {
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	indexCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateOne(
		indexCtx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "project", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	)

	return err
}

// compressGzip compresses data using gzip algorithm.
func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressGzip decompresses gzip-compressed data.
// Returns an error if the decompressed output exceeds maxDecompressedBytes (zip-bomb guard).
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	result, err := io.ReadAll(io.LimitReader(reader, maxDecompressedBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(result)) > maxDecompressedBytes {
		return nil, errors.New("decompressed state exceeds maximum allowed size")
	}
	return result, nil
}

// POST / PUT handler for TF state storage.
func (appServer *server) createTerraformState(ctx *gin.Context) {
	contentType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid Content-Type header"})
		return
	}

	if contentType != "application/json" && contentType != "application/octet-stream" {
		ctx.JSON(
			http.StatusUnsupportedMediaType,
			gin.H{
				"error": "Content-Type must be application/json or application/octet-stream",
			},
		)
		return
	}

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxStateBodyBytes)
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	project := ctx.Param("project")
	username := strings.TrimSpace(ctx.GetString("username"))
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	// Lock ID is mandatory.
	requestLockID := strings.TrimSpace(ctx.Query("ID"))
	if requestLockID == "" {
		requestLockID = strings.TrimSpace(ctx.Query("id"))
	}
	if requestLockID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lock ID is required"})
		return
	}

	compressedBody, err := compressGzip(body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compress state"})
		return
	}

	// Atomically verify the lock and write the state in a single operation.
	// The filter matches only when the document exists and holds the exact lock ID,
	// so no separate lock-check round-trip is needed and no upsert is allowed.
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "state_username", Value: username},
			{Key: "body", Value: bson.Binary{Subtype: 0x00, Data: compressedBody}},
			{Key: "compression_algorithm", Value: "gzip"},
			{Key: "state_update_time", Value: time.Now().UTC()},
		}},
	}

	writeCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := collection.UpdateOne(
		writeCtx,
		bson.D{{Key: "project", Value: project}, {Key: "lock_id", Value: requestLockID}},
		update,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store state"})
		return
	}

	if result.MatchedCount == 0 {
		ctx.JSON(http.StatusLocked, gin.H{"error": "project not found or lock ID mismatch"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Retrieve the project document (state + lock) for a project.
func (appServer *server) findProjectDocument(
	ctx context.Context,
	project string,
) (terraformProjectDocument, error) {
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var document terraformProjectDocument
	err := collection.FindOne(
		findCtx,
		bson.D{{Key: "project", Value: project}},
	).Decode(&document)
	if err != nil {
		return terraformProjectDocument{}, err
	}

	return document, nil
}

// GET - return the current terraform state for a project.
// If a lock ID is supplied as a query parameter (?ID= or ?id=) it is validated
// against the stored lock; requests without a lock ID are served unconditionally.
func (appServer *server) getTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")

	requestLockID := strings.TrimSpace(ctx.Query("ID"))
	if requestLockID == "" {
		requestLockID = strings.TrimSpace(ctx.Query("id"))
	}

	document, err := appServer.findProjectDocument(ctx.Request.Context(), project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load state"})
		return
	}

	if requestLockID != "" && document.LockID != requestLockID {
		ctx.JSON(http.StatusLocked, gin.H{"error": "lock ID mismatch"})
		return
	}

	if len(document.Body.Data) == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state not found"})
		return
	}

	var responseData []byte
	switch document.CompressionAlgorithm {
	case "gzip":
		decompressedData, err := decompressGzip(document.Body.Data)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decompress state"})
			return
		}
		responseData = decompressedData
	case "", "none":
		responseData = document.Body.Data
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("unsupported compression algorithm: %s", document.CompressionAlgorithm)})
		return
	}

	ctx.Data(http.StatusOK, "application/json", responseData)
}

// GET /lock - returns current lock details for a project.
func (appServer *server) getTerraformStateLock(ctx *gin.Context) {
	project := ctx.Param("project")

	projectDoc, err := appServer.findProjectDocument(ctx.Request.Context(), project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lock"})
		return
	}

	if projectDoc.LockID == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"user":             projectDoc.LockUsername,
		"LockID":           projectDoc.LockID,
		"last_update_time": projectDoc.LockTime,
	})
}

// DELETE /lock - clears the lock fields on the project document.
// This operation is idempotent and returns success even when no lock exists.
func (appServer *server) deleteTerraformStateLock(ctx *gin.Context) {
	project := ctx.Param("project")
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	deleteCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	_, err := collection.UpdateOne(
		deleteCtx,
		bson.D{{Key: "project", Value: project}},
		bson.D{{Key: "$unset", Value: bson.D{
			{Key: "lock_id", Value: ""},
			{Key: "lock_username", Value: ""},
			{Key: "lock_time", Value: ""},
		}}},
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// LOCK - atomically acquires a lock on the project document.
// Uses FindOneAndUpdate with upsert so that:
//   - no document for project → upsert creates one with the lock set, returns 200
//   - document exists with no lock → sets lock fields, returns 200
//   - document exists with same lock ID → refreshes lock_time, returns 200
//   - document exists with a different lock ID → upsert hits unique index → 409 Conflict
func (appServer *server) lockTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")
	username := strings.TrimSpace(ctx.GetString("username"))
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxLockBodyBytes)
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var lockRequest terraformLockRequest
	if err := json.Unmarshal(body, &lockRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid lock JSON body"})
		return
	}
	if lockRequest.LockID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lock ID is required"})
		return
	}

	opCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	// Match the project document only when it has no lock or the same lock ID.
	// If a different lock ID is present the filter won't match; the upsert then
	// tries to insert a new document which is blocked by the unique project index,
	// yielding a DuplicateKeyError that we surface as 409 Conflict.
	filter := bson.D{
		{Key: "project", Value: project},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "lock_id", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "lock_id", Value: lockRequest.LockID}},
		}},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "lock_id", Value: lockRequest.LockID},
			{Key: "lock_username", Value: username},
			{Key: "lock_time", Value: time.Now().UTC()},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "project", Value: project},
		}},
	}

	err = collection.FindOneAndUpdate(
		opCtx,
		filter,
		update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Err()

	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			ctx.JSON(http.StatusLocked, gin.H{"error": "terraform state is already locked"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UNLOCK - clears the lock fields after atomically validating the lock ID.
func (appServer *server) unlockTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxLockBodyBytes)
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var lockRequest terraformLockRequest
	if err := json.Unmarshal(body, &lockRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid unlock JSON body"})
		return
	}
	if lockRequest.LockID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "lock ID is required"})
		return
	}

	opCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	err = collection.FindOneAndUpdate(
		opCtx,
		bson.D{
			{Key: "project", Value: project},
			{Key: "lock_id", Value: lockRequest.LockID},
		},
		bson.D{{Key: "$unset", Value: bson.D{
			{Key: "lock_id", Value: ""},
			{Key: "lock_username", Value: ""},
			{Key: "lock_time", Value: ""},
		}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Err()

	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found or lock ID mismatch"})
			return
		}
		log.Printf("failed to remove lock for project %s: %v", project, err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func main() {
	appServer, err := newServer(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := appServer.close(context.Background()); closeErr != nil {
			log.Printf("disconnect mongodb: %v", closeErr)
		}
	}()

	router := setupRouter(appServer)
	srv := &http.Server{
		Addr:              "127.0.0.1:8893",
		Handler:           router,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       5 * time.Minute,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
