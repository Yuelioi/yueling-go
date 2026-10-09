package config

import "slices"

// Clone detaches mutable values so readers and publishers cannot change another snapshot.
func (c Config) Clone() Config {
	c.Bot.SuperUsers = slices.Clone(c.Bot.SuperUsers)
	c.Plugins.Disabled = slices.Clone(c.Plugins.Disabled)
	c.AI.Profiles = slices.Clone(c.AI.Profiles)
	c.Bilibili.NotifyUsers = slices.Clone(c.Bilibili.NotifyUsers)
	c.Bilibili.Keywords = slices.Clone(c.Bilibili.Keywords)
	c.Image.Entry = slices.Clone(c.Image.Entry)
	for i := range c.Image.Entry {
		entry := &c.Image.Entry[i]
		entry.Call = slices.Clone(entry.Call)
		if entry.Arg != nil {
			arg := *entry.Arg
			entry.Arg = &arg
		}
	}
	return c
}
