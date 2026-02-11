package tui

import (
	"github.com/charmbracelet/bubbles/list"
)

// MenuItem represents a single menu item
type MenuItem struct {
	title       string
	description string
	alias       string
}

// Title returns the menu item title (implements list.Item)
func (i MenuItem) Title() string { return i.title }

// Description returns the menu item description (implements list.Item)
func (i MenuItem) Description() string { return i.description }

// FilterValue returns the filter value (implements list.Item)
func (i MenuItem) FilterValue() string { return i.title }

// Alias returns the menu item alias
func (i MenuItem) Alias() string { return i.alias }

// DefaultMenuItems returns the default menu items
func DefaultMenuItems() []list.Item {
	return []list.Item{
		MenuItem{
			title:       "Environment",
			description: "Create and manage Kubernetes environments",
			alias:       "env",
		},
		MenuItem{
			title:       "Configuration",
			description: "Manage Crossplane configurations",
			alias:       "cfg",
		},
		MenuItem{
			title:       "Resource",
			description: "Manage Crossplane resources",
			alias:       "res",
		},
		MenuItem{
			title:       "Registry",
			description: "Manage package registries",
			alias:       "reg",
		},
		MenuItem{
			title:       "Provider",
			description: "Manage Crossplane providers",
			alias:       "prv",
		},
		MenuItem{
			title:       "Function",
			description: "Manage Crossplane functions",
			alias:       "fnc",
		},
	}
}
