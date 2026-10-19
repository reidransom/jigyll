package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/reidransom/jigyll/docexport"
)

var documentationFinalize = app.Command("documentation-finalize", "Finalize built native documentation after asset revision rewrites")
var documentationAssetMap = documentationFinalize.Flag("asset-map", "JSON object mapping original asset URLs to published revision URLs").Required().String()
var documentationOutput = documentationFinalize.Arg("output-directory", "Built output directory to finalize without loading a source site").Required().String()

func documentationFinalizeCommand() error {
	data, err := os.ReadFile(*documentationAssetMap)
	if err != nil {
		return fmt.Errorf("read documentation asset map: %w", err)
	}
	var assets map[string]string
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&assets); err != nil {
		return fmt.Errorf("decode documentation asset map: %w", err)
	}
	if assets == nil {
		return fmt.Errorf("documentation asset map must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("documentation asset map must contain exactly one JSON object")
	}
	return docexport.Finalize(*documentationOutput, assets)
}
