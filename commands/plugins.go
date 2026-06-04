package commands

import (
	"fmt"

	"github.com/reidransom/jigyll/plugins"
)

var pluginsApp = app.Command("plugins", "List emulated plugins")

func pluginsCommand() error {
	logger.label("Plugins:", "")
	for _, name := range plugins.Names() {
		fmt.Printf("  %s\n", name)
	}
	fmt.Println("\nhttps://github.com/reidransom/jigyll/blob/master/docs/plugins.md describes plugin implementation status.")
	fmt.Println("(This may not accurately describe your installation, if you are running an older version of jigyll.)")
	return nil
}
