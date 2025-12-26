// Copyright (c) Gigamon, Inc.

// Implements the position action, which can be used to position the objects in the
// MS (apps, maps, etc.) so that the UI rendering of the same would be reasonable.

// For now this is exposed as an action to the user, and would be removed once we have
// this automatically taken care by the UI, so that the user does not have to do an additonal
// action to see the deployed sessions in the UI. 

// For now after a apply, the user should do a terrafrom action on this position, to see the
// the MS properly in the UI

package commonactions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/fmclient"
	"terraform-provider-gigamon/internal/commonresources"

)

var _ action.ActionWithConfigure = &Position{}

// Action to populate the position collection for the MS to ensure that it is rendered
// properly in the UI
func NewPosition() action.Action{
	return &Position{}
}

// Position - Sets up the x,y ccordinates for the MS object.
type Position struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

type PositionModel struct {
	MonitoringSessionIds []types.String `tfsdk:"monitoring_session_ids"`
}

// Datastrucutre to handle the positions and the adjacencies

// MonitoringSessionObject is a geneirc struct to hold the ID for any MS object be it 
//  an App, Map, or Tunnel
type MonitoringSessObject struct{
	Id string `json:"id"`
}

// Struct to represent a Link in MS
type LinkEP struct {
	Id string `json:"id"` // Object ID of this endpoint of the link
}

type MonitoringSessLink struct {
	Source LinkEP `json:"source"`
	Destination LinkEP `json:"dest"`
}

// Struct to get the response from the MS
type MonitoringSessResp struct {
	TrafficMaps []MonitoringSessObject `json:"trafficMaps"`
	Applications []MonitoringSessObject `json:"applications"`
	Tunnels []MonitoringSessObject `json:"tunnels"`
	Links []MonitoringSessLink `json:"links"`
}

// Graph represenation of the MS objects
type NodeData struct {
	Sources map[string]struct {} // This are the edges pointing to this node
	Adjacencies map[string]struct {} // Adjacencies of this node. Use a map to simulate a set
	Level int // Level at which this node is present
}

// Type to represent the posistion of a object in the MS display window
type ObjectPos struct {
	Id string `json:"id"` // ID of the object
	X int `json:"x"` // X coordinate value
	Y int `json:"y"` // Y oordinate value
}


func (p *Position) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ms_position"
}

func (p *Position) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
    resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon MS object positioning schema",
		Attributes: map[string]schema.Attribute{
			"monitoring_session_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Required: true,
				MarkdownDescription: "List of Monitoring Sesssion IDs that we want to position",
			},
		},
	}
}

func (p *Position) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon", req.ProviderData),
		)
		return
	}
	p.fmClient = fmClient
	p.fmClient.DumpDetails(ctx)
}

func (p *Position) clearAllPositions(ctx context.Context, msId string) error {
	// First get all the posistions and then clear them all out
	posData := struct {
		Positions []ObjectPos `json:"positions"`
	}{
		Positions: make([]ObjectPos, 0),
	}
	fmResp, err := p.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/positions", msId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return err
	}
	err = json.Unmarshal(fmResp, &posData)
	if err != nil {
		return err
	}

	// Go through the objects and delete all of them
	for _, pos := range posData.Positions{
		_, err := p.fmClient.DoRequest(
			ctx,
			"DELETE",
		    fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/positions/%s", msId, pos.Id),
			map[string]string {"x": fmt.Sprintf("%d", pos.X), "y": fmt.Sprintf("%d", pos.Y)},
			nil,
			nil,
			"",
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Position) getMSGraph(ctx context.Context, msId string) (map[string]*NodeData, error) {

	// Get the objects and links of ths MS
	fmResp := MonitoringSessResp {
		TrafficMaps: make([]MonitoringSessObject,0),
		Applications: make([]MonitoringSessObject, 0),
		Tunnels: make([]MonitoringSessObject, 0),
		Links: make([]MonitoringSessLink, 0),
	}

	err := commonresources.UpdateMSData(
		ctx,
		msId,
		&fmResp,
		p.fmClient,
	)
	if err != nil {
		return nil, err
	}

	// Create the nodes first from the MS objects
	msGraph := make(map[string]*NodeData)
	for _, maps := range fmResp.TrafficMaps {
		msGraph[maps.Id] = &NodeData{
			Adjacencies: make(map[string]struct{}),
			Sources: make(map[string]struct{}),
			Level: 0,
		}
	}
	for _, tun := range fmResp.Tunnels{
		msGraph[tun.Id] = &NodeData{
			Adjacencies: make(map[string]struct{}),
			Sources: make(map[string]struct{}),
			Level: 0,
		}
	}
	for _, app := range fmResp.Applications{
		msGraph[app.Id] = &NodeData{
			Adjacencies: make(map[string]struct{}),
			Sources: make(map[string]struct{}),
			Level: 0,
		}
	}

	for _, link := range fmResp.Links{
		msGraph[link.Source.Id].Adjacencies[link.Destination.Id] = struct{}{}
		msGraph[link.Destination.Id].Sources[link.Source.Id] = struct {} {}
	}

	tflog.Info(ctx, "msGetGraph *******", map[string]any {
	    "msGraph": msGraph,
	})
	return msGraph, nil
}

func (p *Position) getLevels(ctx context.Context, msGraph map[string]*NodeData) error {

	// Walk through all the ingress nodes which have zero source, and then as we walk
	// if any node goes to zero ingress, since we have already walked all the ingress
	// than add this to the list of ingress to walk
	nodeToWalk := make([]string, 0)
	for node, data := range msGraph{
		if len(data.Sources) == 0 {
			nodeToWalk = append(nodeToWalk, node)
		}
	}

	nodeWalked := 0
	for {
		if len(nodeToWalk) == 0 {
			break
		}
		currentNode := nodeToWalk[0]
		nodeToWalk = nodeToWalk[1:]
		nodeWalked += 1
		currentLevel := msGraph[currentNode].Level + 1
		for neighbor, _ := range msGraph[currentNode].Adjacencies{
			if msGraph[neighbor].Level < currentLevel {
				msGraph[neighbor].Level = currentLevel
			}
			delete(msGraph[neighbor].Sources, currentNode)
			if len(msGraph[neighbor].Sources) == 0 {
				nodeToWalk = append(nodeToWalk, neighbor)
			}
		}
	}

	if nodeWalked != len(msGraph) {
		return fmt.Errorf("Graph has loops and hence could not set position")
	}

	tflog.Info(ctx, "msGraph after level set ++++++++", map[string]any {
	    "msGraph": msGraph,
	})
	return nil
}

func (p *Position) setPositions (ctx context.Context, msId string, msGraph map[string]*NodeData) error {
	return nil
}

// Sets up the object position for the given Monitoring Session ID. returns error in case
// of any errors in the backend operations
func (p *Position) handlePosition(ctx context.Context, msId string) error {
	// Get the current set of posisitons if any and delete them all
	err := p.clearAllPositions(ctx, msId)
	if err != nil {
		return err
	}

	// Get the MS object graph as a map of nodeids along with their adjancenise
	msGraph, err := p.getMSGraph(ctx, msId)

	// Get the levels for each node from this graph
	err = p.getLevels(ctx, msGraph)
	if err != nil {
		return err
	}

	// Program the level of each of these objects to the MS
	err = p.setPositions(ctx, msId, msGraph)
	if err != nil {
		return err
	}
	return nil
}


func (p *Position) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {

	var data PositionModel

    resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if p.fmClient == nil{
		resp.Diagnostics.AddError(
			"Error setting posisiton for Monitoring Session",
            fmt.Sprintf("setting positions for monitoring session failed: not initialized"),
		)
		return
	}

	for _, msId := range(data.MonitoringSessionIds) {
		err := p.handlePosition(ctx, msId.ValueString())
		if err != nil {
		    resp.Diagnostics.AddError(
			    "Error setting posisiton for Monitoring Session",
				fmt.Sprintf(
					"setting positions for monitoring session %s failed: %v",
					msId.ValueString(),
					err,
				),
		    )
	    }
	}


	// For now just log and do nothing
	tflog.Info(ctx, "MS Position actino called", nil)
}
