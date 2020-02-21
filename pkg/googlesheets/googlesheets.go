package googlesheets

import (
	"fmt"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/davecgh/go-spew/spew"
	cd "github.com/grafana/google-sheets-datasource/pkg/googlesheets/columndefinition"
	gc "github.com/grafana/google-sheets-datasource/pkg/googlesheets/googleclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	df "github.com/grafana/grafana-plugin-sdk-go/dataframe"
	"github.com/hashicorp/go-hclog"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheets struct {
	Cache  *cache.Cache
	Logger hclog.Logger
}

// Query function
func (gs *GoogleSheets) Query(ctx context.Context, refID string, qm *QueryModel, config *GoogleSheetConfig, timeRange backend.TimeRange) (*df.Frame, error) {
	client, err := gc.New(ctx, gc.NewAuth(config.ApiKey, config.AuthType, config.JWT))
	if err != nil {
		return df.New(refID), fmt.Errorf("Unable to create service: %v", err.Error())
	}

	sheet, meta, err := gs.getSpreadSheet(client, qm)
	if err != nil {
		return df.New(refID), err
	}

	frame, err := gs.transformSheetToDataFrame(sheet, meta, refID, qm)
	if err != nil {
		return df.New(refID), err
	}

	return frame, nil
}

// TestAPI function
func (gs *GoogleSheets) TestAPI(ctx context.Context, config *GoogleSheetConfig) (*df.Frame, error) {
	_, err := gc.New(ctx, gc.NewAuth(config.ApiKey, config.AuthType, config.JWT))
	return df.New("TestAPI"), err
}

//GetSpreadsheets
func (gs *GoogleSheets) GetSpreadsheetsByServiceAccount(ctx context.Context, config *GoogleSheetConfig) (map[string]string, error) {
	client, err := gc.New(ctx, gc.NewAuth(config.ApiKey, config.AuthType, config.JWT))
	if err != nil {
		return nil, fmt.Errorf("Invalid datasource configuration: %s", err)
	}

	files, err := client.GetSpreadsheetFiles()
	if err != nil {
		return nil, fmt.Errorf("Could not get all files: %s", err.Error())
	}

	fileNames := map[string]string{}
	for _, i := range files {
		fileNames[i.Id] = i.Name
	}

	return fileNames, nil
}

func (gs *GoogleSheets) getSpreadSheet(client client, qm *QueryModel) (*sheets.GridData, map[string]interface{}, error) {
	cacheKey := qm.Spreadsheet.ID + qm.Range
	if item, expires, found := gs.Cache.GetWithExpiration(cacheKey); found && qm.CacheDurationSeconds > 0 {
		return item.(*sheets.GridData), map[string]interface{}{"hit": true, "count": gs.Cache.ItemCount(), "expires": fmt.Sprintf("%vs", int(expires.Sub(time.Now()).Seconds()))}, nil
	}

	result, err := client.GetSpreadsheet(qm.Spreadsheet.ID, qm.Range, true)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to get spreadsheet: %v", err.Error())
	}

	if result.Properties.TimeZone != "" {
		loc, err := time.LoadLocation(result.Properties.TimeZone)
		if err != nil {
			return nil, nil, fmt.Errorf("error while loading timezone: %v", err.Error())
		}
		time.Local = loc
	}

	sheet := result.Sheets[0].Data[0]

	if qm.CacheDurationSeconds > 0 {
		gs.Cache.Set(cacheKey, sheet, time.Duration(qm.CacheDurationSeconds)*time.Second)
	}

	return sheet, map[string]interface{}{"hit": false}, nil
}

func (gs *GoogleSheets) transformSheetToDataFrame(sheet *sheets.GridData, meta map[string]interface{}, refID string, qm *QueryModel) (*df.Frame, error) {
	fields := []*df.Field{}
	columns := getColumnDefintions(sheet.RowData)
	warnings := []string{}

	for _, column := range columns {

		var field *df.Field
		switch column.GetType() {
		case "TIME":
			field = df.NewField(column.Header, nil, make([]*time.Time, len(sheet.RowData)-1))
		case "NUMBER":
			field = df.NewField(column.Header, nil, make([]*float64, len(sheet.RowData)-1))
		case "STRING":
			field = df.NewField(column.Header, nil, make([]*string, len(sheet.RowData)-1))
		}

		field.Config = &df.FieldConfig{Unit: column.GetUnit()}

		if column.HasMixedTypes() {
			warnings = append(warnings, fmt.Sprintf("Multipe data types found in column %s. Using string data type", column.Header))
			gs.Logger.Error("Multipe data types found in column " + column.Header + ". Using string")
		}

		if column.HasMixedUnits() {
			warnings = append(warnings, fmt.Sprintf("Multipe units found in column %s. Formatted value will be used", column.Header))
			gs.Logger.Error("Multipe units found in column " + column.Header + ". Formatted value will be used")
		}

		fields = append(fields, field)
	}

	frame := df.New(refID,
		fields...,
	)

	for rowIndex := 1; rowIndex < len(sheet.RowData); rowIndex++ {
		for columnIndex, cellData := range sheet.RowData[rowIndex].Values {
			if columnIndex >= len(columns) {
				continue
			}

			switch columns[columnIndex].GetType() {
			case "TIME":
				time, err := dateparse.ParseLocal(cellData.FormattedValue)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("Error while parsing date at row %v in column %s", rowIndex+1, columns[columnIndex].Header))
				} else {
					frame.Fields[columnIndex].Vector.Set(rowIndex-1, &time)
				}
			case "NUMBER":
				if cellData.EffectiveValue != nil {
					frame.Fields[columnIndex].Vector.Set(rowIndex-1, &cellData.EffectiveValue.NumberValue)
				}
			case "STRING":
				frame.Fields[columnIndex].Vector.Set(rowIndex-1, &cellData.FormattedValue)
			}
		}
	}

	meta["warnings"] = warnings
	meta["spreadsheetId"] = qm.Spreadsheet.ID
	meta["range"] = qm.Range
	frame.Meta = &df.QueryResultMeta{Custom: meta}
	gs.Logger.Debug("frame.Meta", spew.Sdump(frame.Meta))

	return frame, nil
}

func getUniqueColumnName(formattedName string, columnIndex int, columns map[string]bool) string {
	name := formattedName
	if name == "" {
		name = fmt.Sprintf("Field %v", columnIndex+1)
	}

	nameExist := true
	counter := 1
	for nameExist {
		if _, exist := columns[name]; exist {
			name = fmt.Sprintf("%s%v", formattedName, counter)
			counter++
		} else {
			nameExist = false
		}
	}

	return name
}

func getColumnDefintions(rows []*sheets.RowData) []*cd.ColumnDefinition {
	columns := []*cd.ColumnDefinition{}
	columnMap := map[string]bool{}
	headerRow := rows[0].Values

	for columnIndex, headerCell := range headerRow {
		name := getUniqueColumnName(strings.TrimSpace(headerCell.FormattedValue), columnIndex, columnMap)
		columnMap[name] = true
		columns = append(columns, cd.New(name, columnIndex))
	}

	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		for _, column := range columns {
			if column.ColumnIndex < len(rows[rowIndex].Values) {
				column.CheckCell(rows[rowIndex].Values[column.ColumnIndex])
			}
		}
	}

	return columns
}