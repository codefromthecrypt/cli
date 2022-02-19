package cli

type UpgradeCmd struct {
}

func (c *UpgradeCmd) Run(ctx *Context) error {
	homeDir, err := ensureHomeDirectory()
	if err != nil {
		return err
	}

	return checkDependencies(homeDir, true)
}
