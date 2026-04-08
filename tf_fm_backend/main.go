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
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// All state and lock documents for every project are stored in the single shared
// collection "terraformBackendState". The doc_type field identifies the kind of document
// and project identifies the TF project. A unique compound index on (doc_type, project)
// enforces exactly one state document and one lock document per project.

const (
	defaultMongoURI     = "mongodb://localhost:27017/?replicaSet=fmreplica"
	databaseName        = "fmdb2"
	backendCollection   = "terraformBackendState"
	userTokenCollection = "fmUserTokens"
	groupCollection     = "fmGroups"
	roleCollection      = "fmRoles"
	stateDocType        = "terraform_state"
	stateLockType       = "tf_state_lock"
)

var errUnauthorizedUser = errors.New("unauthorized user")

type server struct {
	mongoClient *mongo.Client
}

// State document retrieved from MongoDB.
type terraformStateDocument struct {
	MongoID              bson.ObjectID `bson:"_id,omitempty"`
	DocType              string        `bson:"doc_type"`
	Project              string        `bson:"project"`
	Username             string        `bson:"username"`
	Body                 bson.Binary   `bson:"body"`
	CompressionAlgorithm string        `bson:"compression_algorithm"`
	LastUpdateTime       time.Time     `bson:"last_update_time"`
}

// Lock document retrieved from MongoDB.
type terraformLockDocument struct {
	MongoID        bson.ObjectID `bson:"_id,omitempty"`
	DocType        string        `bson:"doc_type"`
	Project        string        `bson:"project"`
	Username       string        `bson:"username"`
	LockID         string        `bson:"ID"`
	LastUpdateTime time.Time     `bson:"last_update_time"`
}

// Lock/Unlock request body sent by TF.
type terraformLockRequest struct {
	LockID string `json:"ID"`
}

// Below is not a full representation of the documents in these collections, but rather
// only the fields which are relevant to our validation

// For a user to be able to use TF and the backend Mongodb they should be
// allowed write access to TRAFFIC_CONTROL.
// In FM the linkage is as follows
// Token -> provides a set of groups
//   Group -> Provides a set of Roles
//      Role -> Provides a set of object (TRAFFIC_CNTOROL etc) and permission All/write etc

type userTokenDocument struct {
	Token    string   `bson:"token"`
	Username string   `bson:"username"`
	ExpiryTs int64    `bson:"expiryTs"`
	Groups   []string `bson:"groups"`
}

type groupDocument struct {
	Name      string   `bson:"name"`
	RoleUUIDs []string `bson:"roleUUIDs"`
}

type roleScopeDocument struct {
	Type    string   `bson:"type"`
	Actions []string `bson:"actions"`
}

type roleDocument struct {
	UUID  string              `bson:"uuid"`
	Scope []roleScopeDocument `bson:"scope"`
}

// Validates if the owner of this token is authorized to do the operation, i.e. has the minimal
// permission conveyed by requiredPermission. Usually a type of ALL, would give permission for
// for any object, and similarly an action of ALL would allow either read or write actions

func (appServer *server) validateAuthorizationToken(
	ctx context.Context,
	token string,
	requiredPermission [2]string,
) (string, error) {
	database := appServer.mongoClient.Database(databaseName)
	userTokenColl := database.Collection(userTokenCollection)
	groupColl := database.Collection(groupCollection)
	roleColl := database.Collection(roleCollection)

	findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// First get the list of scope for this token by going through the groups/roles collections
	userFilter := bson.D{
		{Key: "token", Value: token},
		{Key: "expiryTs", Value: bson.D{{Key: "$gt", Value: time.Now().UTC().UnixMilli()}}},
	}

	var userToken userTokenDocument
	err := userTokenColl.FindOne(findCtx, userFilter).Decode(&userToken)
	if err != nil {
		return "", err
	}

	if len(userToken.Groups) == 0 {
		return "", errUnauthorizedUser
	}

	groupCursor, err := groupColl.Find(
		findCtx,
		bson.D{{Key: "name", Value: bson.D{{Key: "$in", Value: userToken.Groups}}}},
	)
	if err != nil {
		return "", err
	}
	defer groupCursor.Close(findCtx)

	roleUUIDSet := make(map[string]struct{})
	for groupCursor.Next(findCtx) {
		var group groupDocument
		if err := groupCursor.Decode(&group); err != nil {
			return "", err
		}

		for _, roleUUID := range group.RoleUUIDs {
			roleUUID = strings.TrimSpace(roleUUID)
			if roleUUID == "" {
				continue
			}
			roleUUIDSet[roleUUID] = struct{}{}
		}
	}
	if err := groupCursor.Err(); err != nil {
		return "", err
	}

	if len(roleUUIDSet) == 0 {
		return "", errUnauthorizedUser
	}

	roleUUIDs := make([]string, 0, len(roleUUIDSet))
	for roleUUID := range roleUUIDSet {
		roleUUIDs = append(roleUUIDs, roleUUID)
	}

	roleCursor, err := roleColl.Find(
		findCtx,
		bson.D{{Key: "uuid", Value: bson.D{{Key: "$in", Value: roleUUIDs}}}},
	)
	if err != nil {
		return "", err
	}
	defer roleCursor.Close(findCtx)

	requiredType := strings.TrimSpace(requiredPermission[0])
	requiredAction := strings.TrimSpace(requiredPermission[1])
	if requiredType == "" || requiredAction == "" {
		return "", errUnauthorizedUser
	}

	for roleCursor.Next(findCtx) {
		var role roleDocument
		if err := roleCursor.Decode(&role); err != nil {
			return "", err
		}

		for _, scope := range role.Scope {
			hasAllowedType := strings.EqualFold(scope.Type, "ALL") ||
				strings.EqualFold(scope.Type, requiredType)
			if !hasAllowedType {
				continue
			}

			for _, action := range scope.Actions {
				hasAllowedAction := strings.EqualFold(action, "ALL") ||
					strings.EqualFold(action, requiredAction)
				if hasAllowedAction {
					return userToken.Username, nil
				}
			}

		}
	}
	if err := roleCursor.Err(); err != nil {
		return "", err
	}

	return "", errUnauthorizedUser
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
			if err == mongo.ErrNoDocuments || errors.Is(err, errUnauthorizedUser) {
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

		ctx.Set("token", token)
		ctx.Set("username", username)

		ctx.Next()
	}
}

// List of URL that we provide for the http backend of TF. The project is an instance of
// infra that the customer wants to manage.
func setupRouter(appServer *server) *gin.Engine {
	router := gin.Default()
	defaultPermission := [2]string{"TRAFFIC_CONTROL", "write"}
	adminPermission := [2]string{"ALL", "ALL"}

	router.GET(
		"/terraform-state/:project",
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.getTerraformState,
	)
	router.POST(
		"/terraform-state/:project",
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.createTerraformState,
	)
	router.GET(
		"/terraform-state/:project/lock",
		appServer.requireBearerAuthorization(adminPermission),
		appServer.getTerraformStateLock,
	)
	router.DELETE(
		"/terraform-state/:project/lock",
		appServer.requireBearerAuthorization(adminPermission),
		appServer.deleteTerraformStateLock,
	)
	router.Handle(
		"LOCK",
		"/terraform-state/:project/lock",
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.lockTerraformState,
	)
	router.Handle(
		"UNLOCK",
		"/terraform-state/:project/lock",
		appServer.requireBearerAuthorization(defaultPermission),
		appServer.unlockTerraformState,
	)

	return router
}

// Setup the MongoDB client, verify connectivity, and ensure the shared collection index.
func newServer(ctx context.Context) (*server, error) {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = defaultMongoURI
	}

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	appServer := &server{mongoClient: client}
	if err := appServer.ensureBackendIndex(ctx); err != nil {
		return nil, fmt.Errorf("ensure backend index: %w", err)
	}

	return appServer, nil
}

func (appServer *server) close(ctx context.Context) error {
	return appServer.mongoClient.Disconnect(ctx)
}

// Create the unique compound index on (doc_type, project) once at startup.
func (appServer *server) ensureBackendIndex(ctx context.Context) error {
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	indexCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateOne(
		indexCtx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "doc_type", Value: 1}, {Key: "project", Value: 1}},
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
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
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

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	project := ctx.Param("project")
	username := strings.TrimSpace(ctx.GetString("username"))
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	// If a lock ID is provided as a query parameter, validate it against the stored lock.
	requestLockID := strings.TrimSpace(ctx.Query("ID"))
	if requestLockID == "" {
		requestLockID = strings.TrimSpace(ctx.Query("id"))
	}
	if requestLockID != "" {
		storedLock, lockErr := appServer.findTerraformStateLock(ctx.Request.Context(), project)
		if lockErr != nil {
			if lockErr == mongo.ErrNoDocuments {
				ctx.JSON(
					http.StatusBadRequest,
					gin.H{"error": "terraform state has no lock"},
				)
				return
			}

			ctx.JSON(
				http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("failed to validate lock: %s", lockErr)},
			)
			return
		}

		if storedLock.LockID != requestLockID {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "lock ID mismatch"})
			return
		}
	}

	compressedBody, err := compressGzip(body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compress state"})
		return
	}

	document := bson.M{
		"doc_type":              stateDocType,
		"project":               project,
		"username":              username,
		"body":                  bson.Binary{Subtype: 0x00, Data: compressedBody},
		"compression_algorithm": "gzip",
		"last_update_time":      time.Now().UTC(),
	}

	insertCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	if _, err := collection.UpdateOne(
		insertCtx,
		bson.D{{Key: "doc_type", Value: stateDocType}, {Key: "project", Value: project}},
		bson.D{{Key: "$set", Value: document}},
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store state"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Retrieve the state document for a project.
func (appServer *server) findTerraformState(
	ctx context.Context,
	project string,
) (terraformStateDocument, error) {
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var document terraformStateDocument
	err := collection.FindOne(
		findCtx,
		bson.D{{Key: "doc_type", Value: stateDocType}, {Key: "project", Value: project}},
	).Decode(&document)
	if err != nil {
		return terraformStateDocument{}, err
	}

	return document, nil
}

// Retrieve the lock document for a project.
func (appServer *server) findTerraformStateLock(
	ctx context.Context,
	project string,
) (terraformLockDocument, error) {
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	findCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var document terraformLockDocument
	err := collection.FindOne(
		findCtx,
		bson.D{{Key: "doc_type", Value: stateLockType}, {Key: "project", Value: project}},
	).Decode(&document)
	if err != nil {
		return terraformLockDocument{}, err
	}

	return document, nil
}

// GET - return the current terraform state for a project.
func (appServer *server) getTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")

	document, err := appServer.findTerraformState(ctx.Request.Context(), project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load state"})
		return
	}

	decompressedData, err := decompressGzip(document.Body.Data)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decompress state"})
		return
	}

	ctx.Data(http.StatusOK, "application/json", decompressedData)
}

// GET /lock - returns current lock details for a project.
func (appServer *server) getTerraformStateLock(ctx *gin.Context) {
	project := ctx.Param("project")

	storedLock, err := appServer.findTerraformStateLock(ctx.Request.Context(), project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"user":             storedLock.Username,
		"LockID":           storedLock.LockID,
		"last_update_time": storedLock.LastUpdateTime,
	})
}

// DELETE /lock - removes the lock document for a project if it exists.
// This operation is idempotent and returns success even when no lock exists.
func (appServer *server) deleteTerraformStateLock(ctx *gin.Context) {
	project := ctx.Param("project")
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	deleteCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	_, err := collection.DeleteOne(
		deleteCtx,
		bson.D{{Key: "doc_type", Value: stateLockType}, {Key: "project", Value: project}},
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DELETE - removes the state document for a project.
func (appServer *server) deleteTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	deleteCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	collection.DeleteOne(
		deleteCtx,
		bson.D{{Key: "doc_type", Value: stateDocType}, {Key: "project", Value: project}},
	)

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// LOCK - atomically creates or refreshes a lock document for the project.
// Uses FindOneAndUpdate with upsert so that:
//   - matching doc (same doc_type + project + ID) → updates last_update_time, returns 200
//   - no matching doc but doc_type+project already taken → upsert hits unique index → 423
//   - no matching doc and no existing lock → inserts new document, returns 200
func (appServer *server) lockTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")
	username := strings.TrimSpace(ctx.GetString("username"))
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
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

	filter := bson.D{
		{Key: "doc_type", Value: stateLockType},
		{Key: "project", Value: project},
		{Key: "ID", Value: lockRequest.LockID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "username", Value: username},
			{Key: "last_update_time", Value: time.Now().UTC()},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "doc_type", Value: stateLockType},
			{Key: "project", Value: project},
			{Key: "ID", Value: lockRequest.LockID},
		}},
	}

	err = collection.FindOneAndUpdate(
		opCtx,
		filter,
		update,
		options.FindOneAndUpdate().SetUpsert(true),
	).Err()

	if err != nil && err != mongo.ErrNoDocuments {
		if mongo.IsDuplicateKeyError(err) {
			ctx.JSON(http.StatusLocked, gin.H{"error": "terraform state is already locked"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create lock"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UNLOCK - removes the lock document after validating the lock ID.
func (appServer *server) unlockTerraformState(ctx *gin.Context) {
	project := ctx.Param("project")
	collection := appServer.mongoClient.Database(databaseName).Collection(backendCollection)

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
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

	storedLock, err := appServer.findTerraformStateLock(ctx.Request.Context(), project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load lock"})
		return
	}

	if lockRequest.LockID != storedLock.LockID {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "lock ID mismatch",
			"parameters": gin.H{
				"ID": "does not match existing lock",
			},
		})
		return
	}

	deleteCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := collection.DeleteOne(
		deleteCtx,
		bson.D{
			{Key: "_id", Value: storedLock.MongoID},
			{Key: "doc_type", Value: stateLockType},
			{Key: "project", Value: project},
			{Key: "ID", Value: lockRequest.LockID},
		},
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove lock"})
		return
	}

	if result.DeletedCount == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "terraform state lock not found"})
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8893"
	}

	if err := router.Run("127.0.0.1:" + port); err != nil {
		log.Fatal(err)
	}
}
