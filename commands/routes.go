package commands

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/reidransom/gojekyll/site"
)

var routes = app.Command("routes", "Display site permalinks and associated files")
var dynamicRoutes = routes.Flag("dynamic", "Only show routes to non-static files").Bool()
var jsonRoutes = routes.Flag("json", "Output routes in JSON format").Bool()

func routesCommand(site *site.Site) error {
	if *jsonRoutes {
		// Create a map for JSON output
		routeMap := make(map[string]string)
		for u, p := range site.Routes {
			if !*dynamicRoutes || !p.IsStatic() {
				routeMap[u] = p.Source()
			}
		}
		
		// Output JSON
		jsonData, err := json.MarshalIndent(routeMap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
	} else {
		// Original text output
		logger.label("Routes:", "")
		var urls []string
		for u, p := range site.Routes {
			if !*dynamicRoutes || !p.IsStatic() {
				urls = append(urls, u)
			}
		}
		sort.Strings(urls)
		for _, u := range urls {
			filename := site.Routes[u].Source()
			fmt.Printf("  %s -> %s\n", u, filename)
		}
	}
	return nil
}
