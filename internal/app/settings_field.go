package app

import "github.com/egoist/mygo/ui"

func settingsField(c *ui.Context, label, description string, control func() ui.Element) {
	c.Key(label)
	ui.Field(c, label, func() { control().WidthPercent(100) }).Description(description)
}
