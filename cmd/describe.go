package cmd

import (
	"errors"
	"github.com/spf13/cobra"
	"github.com/wish/ctl/cmd/util/parsing"
	"github.com/wish/ctl/pkg/client"
	"github.com/wish/ctl/pkg/client/types"
)

var supportedDescribeTypes = [][]string{
	{"pods", "pod", "po"},
}

func describeCmd(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe pods [flags]",
		Short: "Show details of a specific pod(s)",
		Long: `Print a detailed description of the pods specified by name.
If namespace not specified, it will get all the pods across all the namespaces.
If context(s) not specified, it will search through all contexts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxs, _ := cmd.Flags().GetStringSlice("context")
			namespace, _ := cmd.Flags().GetString("namespace")
			options, err := parsing.ListOptions(cmd)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				defer cmd.Help()
				return errors.New("no resource type provided")
			}

			switch args[0] {
			case "pods", "pod", "po":
				var pods []types.PodDiscovery
				var err error
				if len(args) == 1 {
					pods, err = c.ListPodsOverContexts(ctxs, namespace, options)
				} else {
					pods, err = c.FindPods(ctxs, namespace, args[1:], options)
				}
				if err != nil {
					return err
				}
				if len(pods) == 0 {
					return errors.New("could not find any matching pods")
				}
				describePodList(pods)
			default:
				defer cmd.Help()
				return errors.New(`The resource type "` + args[0] + `" was not found.
See 'ctl describe'`)
			}
			return nil
		},
	}

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		cmd.Println("Choose from the list of supported resources:")
		for _, names := range supportedGetTypes {
			cmd.Printf(" * %s\n", names[0])
		}
		return nil
	})

	return cmd
}
