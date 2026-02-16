package switcher

type DeleteResult struct {
	DeletedVersion   string
	WasActive        bool
	SwitchedToNewest bool
	ActiveAfter      ActiveVersion
	ToolSyncWarning  string
}
