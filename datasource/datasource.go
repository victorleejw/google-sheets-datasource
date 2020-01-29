package main

import (
	"encoding/json"
	"fmt"

	gs "github.com/grafana/google-sheets-datasource/datasource/googlesheets"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	df "github.com/grafana/grafana-plugin-sdk-go/dataframe"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"

	"golang.org/x/net/context"
)

const (
	pluginID     = "google-sheets-datasource"
	variableName = "GOOGLE_SHEETS_API_KEY"
)

var pluginLogger = hclog.New(&hclog.LoggerOptions{
	Name:  pluginID,
	Level: hclog.LevelFromString("DEBUG"),
})

func main() {
	err := backend.Serve(backend.ServeOpts{
		DataQueryHandler: &GoogleSheetsDataSource{
			logger: pluginLogger,
		},
	})
	if err != nil {
		pluginLogger.Error(err.Error())
	}
}

type GoogleSheetsDataSource struct {
	plugin.NetRPCUnsupportedPlugin
	logger hclog.Logger
}

func (gsd *GoogleSheetsDataSource) DataQuery(ctx context.Context, req *backend.DataQueryRequest) (*backend.DataQueryResponse, error) {
	res := &backend.DataQueryResponse{}
	config := gs.GoogleSheetConfig{}
	err := json.Unmarshal(req.PluginConfig.JSONData, &config)
	if err != nil {
		gsd.logger.Error("Could not unmarshal DataSourceInfo json", "Error", err)
		return nil, err
	}

	for _, q := range req.Queries {
		queryModel := &gs.QueryModel{}
		err := json.Unmarshal(q.JSON, &queryModel)

		if err != nil {
			gsd.logger.Error("Failed to unmarshal query: %v", err.Error())
			return nil, fmt.Errorf("Invalid query")
		}

		var frame *df.Frame
		switch queryModel.QueryType {
		case "testAPI":
			gsd.logger.Debug("FailedFailedFailedFailed: %v", config.JwtFile)
			frame, err = gs.TestAPI(ctx, &config)
		case "query":
			gsd.logger.Debug("FailedFailedFailedFailed: %v", string([]byte(config.JwtFile)))
			frame, err = gs.Query(ctx, q.RefID, queryModel, &config, gsd.logger)
		default:
			return nil, fmt.Errorf("Invalid query type")
		}

		if err != nil {
			gsd.logger.Debug("QueryError", "QueryError", err.Error())
			frame := df.New("default")
			frame.RefID = q.RefID
			frame.Meta = &df.QueryResultMeta{Custom: make(map[string]interface{})}
			frame.Meta.Custom["error"] = err.Error()
			// return nil, err
		}

		res.Frames = append(res.Frames, frame)
	}

	return res, nil
}
