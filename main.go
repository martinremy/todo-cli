package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var filePath string

func resolveFile() string {
	if filePath != "" {
		return filePath
	}
	if env := os.Getenv("TODO_FILE"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".todo.jsonl")
}

func main() {
	root := &cobra.Command{
		Use:   "todo",
		Short: "A personal CLI todo manager backed by JSONL",
	}
	root.PersistentFlags().StringVar(&filePath, "file", "", "path to JSONL data file")

	root.AddCommand(addCmd(), listCmd(), updateCmd(), doneCmd(), rmCmd(), categoriesCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func addCmd() *cobra.Command {
	var desc, category, due, recurrence string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new todo",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.Join(args, " ")

			var dueTime time.Time
			if due != "" {
				parsed, err := time.Parse("2006-01-02", due)
				if err != nil {
					return fmt.Errorf("bad --due format (use YYYY-MM-DD): %w", err)
				}
				dueTime = parsed
			} else {
				dueTime = time.Now().UTC().AddDate(0, 0, 14)
			}

			t := NewTodo(name, strPtr(desc), strPtr(category), dueTime, strPtr(recurrence))
			if err := AppendTodo(resolveFile(), t); err != nil {
				return err
			}
			fmt.Printf("Added: %s %s\n", t.ID[:12], t.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "description")
	cmd.Flags().StringVar(&category, "category", "", "category")
	cmd.Flags().StringVar(&due, "due", "", "due date (YYYY-MM-DD, default: 14 days)")
	cmd.Flags().StringVar(&recurrence, "recurrence", "", "recurrence interval (e.g. 7d, 2w, 1m)")
	return cmd
}

func listCmd() *cobra.Command {
	var all, overdue bool
	var status, category, from, to string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List todos",
		RunE: func(cmd *cobra.Command, args []string) error {
			if overdue && (from != "" || to != "") {
				return fmt.Errorf("--overdue and --from/--to are mutually exclusive")
			}

			path := resolveFile()
			todos, err := ReadTodos(path)
			if err != nil {
				return err
			}

			if all {
				archived, err := ReadTodos(ArchivePath(path))
				if err != nil {
					return err
				}
				todos = append(todos, archived...)
			}

			var statusPtr *Status
			if status != "" {
				s, err := ValidStatus(status)
				if err != nil {
					return err
				}
				statusPtr = &s
			}

			var fromTime, toTime *time.Time
			if from != "" {
				t, err := time.Parse("2006-01-02", from)
				if err != nil {
					return fmt.Errorf("invalid --from date %q (use YYYY-MM-DD)", from)
				}
				fromTime = &t
			}
			if to != "" {
				t, err := time.Parse("2006-01-02", to)
				if err != nil {
					return fmt.Errorf("invalid --to date %q (use YYYY-MM-DD)", to)
				}
				toTime = &t
			}

			filtered := FilterTodos(todos, all, statusPtr, strPtr(category), overdue, fromTime, toTime)
			SortByDue(filtered)

			enc := json.NewEncoder(os.Stdout)
			for _, t := range filtered {
				if err := enc.Encode(t); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "show all items including done")
	cmd.Flags().BoolVar(&overdue, "overdue", false, "show only overdue items")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&category, "category", "", "filter by category")
	cmd.Flags().StringVar(&from, "from", "", "show items due on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "show items due before this date (YYYY-MM-DD)")
	return cmd
}

func updateCmd() *cobra.Command {
	var name, desc, category, statusStr, due, recurrence string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveFile()
			todos, err := ReadTodos(path)
			if err != nil {
				return err
			}
			t, err := FindByID(todos, args[0])
			if err != nil {
				return err
			}

			now := time.Now().UTC().Format(time.RFC3339)
			changed := false

			if cmd.Flags().Changed("name") {
				t.Name = name
				changed = true
			}
			if cmd.Flags().Changed("desc") {
				t.Description = strPtr(desc)
				changed = true
			}
			if cmd.Flags().Changed("category") {
				t.Category = strPtr(category)
				changed = true
			}
			if cmd.Flags().Changed("status") {
				s, err := ValidStatus(statusStr)
				if err != nil {
					return err
				}
				t.Status = s
				changed = true
			}
			if cmd.Flags().Changed("due") {
				parsed, err := time.Parse("2006-01-02", due)
				if err != nil {
					return fmt.Errorf("bad --due format (use YYYY-MM-DD): %w", err)
				}
				t.Due = parsed.UTC().Format(time.RFC3339)
				changed = true
			}
			if cmd.Flags().Changed("recurrence") {
				t.Recurrence = strPtr(recurrence)
				changed = true
			}

			if !changed {
				fmt.Println("Nothing to update.")
				return nil
			}
			t.Updated = now

			// Write back updated list
			for i, item := range todos {
				if item.ID == t.ID {
					todos[i] = *t
					break
				}
			}
			if err := WriteTodos(path, todos); err != nil {
				return err
			}
			fmt.Printf("Updated: %s %s\n", t.ID[:12], t.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&desc, "desc", "", "new description")
	cmd.Flags().StringVar(&category, "category", "", "new category")
	cmd.Flags().StringVar(&statusStr, "status", "", "new status")
	cmd.Flags().StringVar(&due, "due", "", "new due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&recurrence, "recurrence", "", "new recurrence")
	return cmd
}

func doneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a todo as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveFile()
			todos, err := ReadTodos(path)
			if err != nil {
				return err
			}
			t, err := FindByID(todos, args[0])
			if err != nil {
				return err
			}

			next := CompleteTodo(t)

			// Archive the completed item
			if err := AppendTodo(ArchivePath(path), *t); err != nil {
				return fmt.Errorf("archiving: %w", err)
			}

			// Remove completed item from main file, add recurring if any
			var remaining []Todo
			for _, item := range todos {
				if item.ID != t.ID {
					remaining = append(remaining, item)
				}
			}
			if next != nil {
				remaining = append(remaining, *next)
				fmt.Printf("Recurring: created %s %s (due %s)\n",
					next.ID[:12], next.Name, next.Due[:10])
			}

			if err := WriteTodos(path, remaining); err != nil {
				return err
			}
			fmt.Printf("Done: %s %s (archived)\n", t.ID[:12], t.Name)
			return nil
		},
	}
}

func rmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a todo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := resolveFile()
			todos, err := ReadTodos(path)
			if err != nil {
				return err
			}
			t, err := FindByID(todos, args[0])
			if err != nil {
				return err
			}

			if !force {
				fmt.Printf("Remove %q? [y/N] ", t.Name)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(answer)) != "y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			var remaining []Todo
			for _, item := range todos {
				if item.ID != t.ID {
					remaining = append(remaining, item)
				}
			}
			if err := WriteTodos(path, remaining); err != nil {
				return err
			}
			fmt.Printf("Removed: %s %s\n", t.ID[:12], t.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation")
	return cmd
}

func categoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "List distinct categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			todos, err := ReadTodos(resolveFile())
			if err != nil {
				return err
			}
			cats := Categories(todos)
			if len(cats) == 0 {
				fmt.Println("No categories found.")
				return nil
			}
			for _, c := range cats {
				fmt.Println(c)
			}
			return nil
		},
	}
}
