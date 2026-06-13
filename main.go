package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

const editMinArgs = 2

func main() {
	if err := buildApp().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func addAction(_ context.Context, cmd *cli.Command) error {
	text := cmd.Args().First()
	if text == "" {
		return fmt.Errorf("task text is required")
	}

	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	prefix, err := tl.Add(text)
	if err != nil {
		return handleTaskError(err)
	}

	if err := tl.Write(cmd.Root().Bool("delete-if-empty")); err != nil {
		return handleTaskError(err)
	}

	fmt.Println(prefix)

	return nil
}

func listAction(_ context.Context, cmd *cli.Command) error {
	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	tl.List(os.Stdout, "tasks", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))

	return nil
}

func doneAction(_ context.Context, cmd *cli.Command) error {
	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	tl.List(os.Stdout, "done", cmd.Bool("verbose"), cmd.Bool("quiet"), cmd.String("grep"))

	return nil
}

func finishAction(_ context.Context, cmd *cli.Command) error {
	prefix := cmd.Args().First()
	if prefix == "" {
		return fmt.Errorf("task prefix is required")
	}

	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	if err := tl.Finish(prefix); err != nil {
		return handleTaskError(err)
	}

	return tl.Write(cmd.Root().Bool("delete-if-empty"))
}

func removeAction(_ context.Context, cmd *cli.Command) error {
	prefix := cmd.Args().First()
	if prefix == "" {
		return fmt.Errorf("task prefix is required")
	}

	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	if err := tl.Remove(prefix); err != nil {
		return handleTaskError(err)
	}

	return tl.Write(cmd.Root().Bool("delete-if-empty"))
}

func editAction(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < editMinArgs {
		return fmt.Errorf("usage: edit TASK NEW_TEXT")
	}

	prefix := args.Get(0)
	newText := args.Get(1)

	tl, err := loadTaskList(cmd)
	if err != nil {
		return handleTaskError(err)
	}

	if err := tl.Edit(prefix, newText); err != nil {
		return handleTaskError(err)
	}

	return tl.Write(cmd.Root().Bool("delete-if-empty"))
}

// buildApp constructs and returns the CLI app.
// Extracted so tests can call it directly.
func buildApp() *cli.Command {
	return &cli.Command{
		Name:           "tgo",
		Usage:          "A simple task manager",
		DefaultCommand: "list",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "task-dir",
				Aliases: []string{"t"},
				Usage:   "work on the lists in `DIR`",
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "work on `LIST`",
				Value:   "tasks",
			},
			&cli.BoolFlag{
				Name:    "delete-if-empty",
				Aliases: []string{"d"},
				Usage:   "delete the task file if it becomes empty",
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "Add a new task",
				ArgsUsage: "TEXT",
				Action:    addAction,
			},
			{
				Name:   "list",
				Usage:  "List open tasks",
				Flags:  listFlags(),
				Action: listAction,
			},
			{
				Name:   "done",
				Usage:  "List finished tasks",
				Flags:  listFlags(),
				Action: doneAction,
			},
			{
				Name:      "finish",
				Usage:     "Mark a task as finished",
				ArgsUsage: "TASK",
				Action:    finishAction,
			},
			{
				Name:      "remove",
				Usage:     "Remove a task from the list",
				ArgsUsage: "TASK",
				Action:    removeAction,
			},
			{
				Name:      "edit",
				Usage:     "Edit a task's text",
				ArgsUsage: "TASK NEW_TEXT",
				Action:    editAction,
			},
		},
	}
}

// listFlags returns the shared flags for the list and done subcommands.
func listFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "grep",
			Aliases: []string{"g"},
			Usage:   "print only tasks containing `WORD`",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "print full task IDs",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "suppress task IDs in output",
		},
	}
}

// loadTaskList creates a TaskList using the global flags from the root command.
func loadTaskList(cmd *cli.Command) (*TaskList, error) {
	root := cmd.Root()

	return NewTaskList(root.String("task-dir"), root.String("list"))
}

// handleTaskError formats task-specific errors for CLI output.
func handleTaskError(err error) error {
	var (
		ae *ErrAmbiguousPrefix
		ue *ErrUnknownPrefix
		ie *ErrInvalidTaskFile
		be *ErrBadFile
	)

	switch {
	case errors.As(err, &ae):
		return fmt.Errorf("the ID %q matches more than one task", ae.Prefix)
	case errors.As(err, &ue):
		return fmt.Errorf("the ID %q does not match any task", ue.Prefix)
	case errors.As(err, &ie):
		return fmt.Errorf("task file path is a directory: %s", ie.Path)
	case errors.As(err, &be):
		return fmt.Errorf("%s: %s", be.Problem, be.Path)
	default:
		return err
	}
}
