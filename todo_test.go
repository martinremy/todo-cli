package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTodoDefaults(t *testing.T) {
	due := time.Now().Add(14 * 24 * time.Hour).Format("2006-01-02")
	todo := NewTodo("test item", nil, nil, due, nil)
	if todo.Name != "test item" {
		t.Fatalf("expected name 'test item', got %q", todo.Name)
	}
	if todo.Status != StatusTodo {
		t.Fatalf("expected status todo, got %q", todo.Status)
	}
	if todo.Description != nil {
		t.Fatalf("expected nil description")
	}
	if len(todo.ID) != 26 {
		t.Fatalf("expected 26-char ULID, got %d chars", len(todo.ID))
	}
	if todo.Due != due {
		t.Fatalf("expected due %q, got %q", due, todo.Due)
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	t1 := NewTodo("first", strPtr("desc1"), strPtr("work"), "2026-03-01", nil)
	t2 := NewTodo("second", nil, nil, "2026-03-02", nil)

	if err := AppendTodo(path, t1); err != nil {
		t.Fatal(err)
	}
	if err := AppendTodo(path, t2); err != nil {
		t.Fatal(err)
	}

	todos, err := ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}
	if todos[0].Name != "first" || todos[1].Name != "second" {
		t.Fatal("names don't match")
	}
	if todos[0].Description == nil || *todos[0].Description != "desc1" {
		t.Fatal("description mismatch")
	}
}

func TestReadNonexistentFile(t *testing.T) {
	todos, err := ReadTodos("/tmp/nonexistent-todo-test-file.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 0 {
		t.Fatal("expected empty list")
	}
}

func TestFindByID(t *testing.T) {
	t1 := NewTodo("item1", nil, nil, "2026-03-01", nil)
	todos := []Todo{t1}

	found, err := FindByID(todos, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != t1.ID {
		t.Fatal("wrong item found")
	}

	_, err = FindByID(todos, "01ZZZZZZZZZZZZZZZZZZZZZZZZ")
	if err == nil {
		t.Fatal("expected error for non-matching ID")
	}
}

func TestFilterExcludesDone(t *testing.T) {
	t1 := NewTodo("active", nil, nil, "2026-03-01", nil)
	t2 := NewTodo("finished", nil, nil, "2026-03-01", nil)
	t2.Status = StatusDone

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, nil, false, nil, nil)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-done item, got %d", len(filtered))
	}
	if filtered[0].Name != "active" {
		t.Fatal("wrong item")
	}
}

func TestFilterAll(t *testing.T) {
	t1 := NewTodo("active", nil, nil, "2026-03-01", nil)
	t2 := NewTodo("finished", nil, nil, "2026-03-01", nil)
	t2.Status = StatusDone

	filtered := FilterTodos([]Todo{t1, t2}, true, nil, nil, false, nil, nil)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items with --all, got %d", len(filtered))
	}
}

func TestListAllIncludesArchived(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	archivePath := ArchivePath(path)

	active := NewTodo("active task", nil, strPtr("work"), "2026-03-01", nil)
	done := NewTodo("done task", nil, strPtr("work"), "2026-03-02", nil)
	CompleteTodo(&done)

	// Active item in main file, done item in archive
	if err := AppendTodo(path, active); err != nil {
		t.Fatal(err)
	}
	if err := AppendTodo(archivePath, done); err != nil {
		t.Fatal(err)
	}

	// Without archive: only active item
	activeTodos, err := ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	filtered := FilterTodos(activeTodos, false, nil, nil, false, nil, nil)
	if len(filtered) != 1 || filtered[0].Name != "active task" {
		t.Fatalf("expected only active task, got %d items", len(filtered))
	}

	// With archive (simulating --all): both items
	archived, err := ReadTodos(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	all := append(activeTodos, archived...)
	filteredAll := FilterTodos(all, true, nil, nil, false, nil, nil)
	if len(filteredAll) != 2 {
		t.Fatalf("expected 2 items with --all, got %d", len(filteredAll))
	}

	// Verify both are present
	names := map[string]bool{}
	for _, item := range filteredAll {
		names[item.Name] = true
	}
	if !names["active task"] || !names["done task"] {
		t.Fatalf("expected both tasks, got %v", names)
	}
}

func TestFilterByCategory(t *testing.T) {
	cat := "work"
	t1 := NewTodo("item1", nil, &cat, "2026-03-01", nil)
	t2 := NewTodo("item2", nil, nil, "2026-03-01", nil)

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, &cat, false, nil, nil)
	if len(filtered) != 1 || filtered[0].Name != "item1" {
		t.Fatal("category filter failed")
	}
}

func TestFilterOverdue(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour).Format("2006-01-02")
	future := time.Now().Add(48 * time.Hour).Format("2006-01-02")
	t1 := NewTodo("overdue", nil, nil, past, nil)
	t2 := NewTodo("upcoming", nil, nil, future, nil)

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, nil, true, nil, nil)
	if len(filtered) != 1 || filtered[0].Name != "overdue" {
		t.Fatal("overdue filter failed")
	}
}

func TestCompleteTodoNoRecurrence(t *testing.T) {
	todo := NewTodo("one-off", nil, nil, "2026-03-01", nil)
	next := CompleteTodo(&todo)
	if todo.Status != StatusDone {
		t.Fatal("expected done status")
	}
	if next != nil {
		t.Fatal("expected no recurring item")
	}
}

func TestCompleteTodoWithRecurrence(t *testing.T) {
	rec := "7d"
	todo := NewTodo("weekly task", nil, strPtr("chores"), "2026-03-01", &rec)
	next := CompleteTodo(&todo)
	if todo.Status != StatusDone {
		t.Fatal("expected done status")
	}
	if next == nil {
		t.Fatal("expected a recurring item")
	}
	if next.Name != "weekly task" {
		t.Fatal("recurring item should have same name")
	}
	if next.Recurrence == nil || *next.Recurrence != "7d" {
		t.Fatal("recurring item should keep recurrence")
	}
	if next.Category == nil || *next.Category != "chores" {
		t.Fatal("recurring item should keep category")
	}
	if next.Status != StatusTodo {
		t.Fatal("recurring item should be todo")
	}

	nextDue, err := time.Parse("2006-01-02", next.Due)
	if err != nil {
		t.Fatal(err)
	}
	if nextDue.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatal("recurring item due date should be ~7 days from now")
	}
}

func TestParseRecurrence(t *testing.T) {
	tests := []struct {
		input string
		days  int
		err   bool
	}{
		{"1d", 1, false},
		{"7d", 7, false},
		{"2w", 14, false},
		{"1m", 30, false},
		{"1y", 365, false},
		{"bad", 0, true},
		{"", 0, true},
		{"3x", 0, true},
	}
	for _, tc := range tests {
		dur, err := ParseRecurrence(tc.input)
		if tc.err {
			if err == nil {
				t.Errorf("expected error for %q", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tc.input, err)
			continue
		}
		expected := time.Duration(tc.days) * 24 * time.Hour
		if dur != expected {
			t.Errorf("%q: expected %v, got %v", tc.input, expected, dur)
		}
	}
}

func TestCategories(t *testing.T) {
	c1 := "work"
	c2 := "personal"
	todos := []Todo{
		NewTodo("a", nil, &c1, "2026-03-01", nil),
		NewTodo("b", nil, &c2, "2026-03-01", nil),
		NewTodo("c", nil, &c1, "2026-03-01", nil),
		NewTodo("d", nil, nil, "2026-03-01", nil),
	}
	cats := Categories(todos)
	if len(cats) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(cats))
	}
	if cats[0] != "personal" || cats[1] != "work" {
		t.Fatalf("expected [personal work], got %v", cats)
	}
}

func TestSortByDue(t *testing.T) {
	t1 := NewTodo("later", nil, nil, "2026-03-03", nil)
	t2 := NewTodo("sooner", nil, nil, "2026-03-02", nil)
	todos := []Todo{t1, t2}
	SortByDue(todos)
	if todos[0].Name != "sooner" {
		t.Fatal("expected sooner first after sort")
	}
}

func TestWriteTodosRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	todos := []Todo{
		NewTodo("a", nil, nil, "2026-03-01", nil),
		NewTodo("b", nil, nil, "2026-03-02", nil),
	}
	if err := WriteTodos(path, todos); err != nil {
		t.Fatal(err)
	}

	// Remove one and rewrite
	if err := WriteTodos(path, todos[:1]); err != nil {
		t.Fatal(err)
	}

	read, err := ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 1 {
		t.Fatalf("expected 1 todo after rewrite, got %d", len(read))
	}
}

func TestArchivePath(t *testing.T) {
	tests := []struct{ in, out string }{
		{"~/.todo.jsonl", "~/.todo.archive.jsonl"},
		{"/data/tasks.jsonl", "/data/tasks.archive.jsonl"},
		{"todos.jsonl", "todos.archive.jsonl"},
	}
	for _, tc := range tests {
		got := ArchivePath(tc.in)
		if got != tc.out {
			t.Errorf("ArchivePath(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

func TestArchiveOnDone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	archivePath := ArchivePath(path)

	rec := "7d"
	t1 := NewTodo("one-off task", nil, strPtr("work"), "2026-03-01", nil)
	t2 := NewTodo("recurring task", nil, strPtr("work"), "2026-03-02", &rec)
	if err := AppendTodo(path, t1); err != nil {
		t.Fatal(err)
	}
	if err := AppendTodo(path, t2); err != nil {
		t.Fatal(err)
	}

	// Complete the one-off task
	todos, err := ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	found, err := FindByID(todos, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	CompleteTodo(found)
	if err := AppendTodo(archivePath, *found); err != nil {
		t.Fatal(err)
	}

	var remaining []Todo
	for _, item := range todos {
		if item.ID != found.ID {
			remaining = append(remaining, item)
		}
	}
	if err := WriteTodos(path, remaining); err != nil {
		t.Fatal(err)
	}

	// Verify main file has only the recurring task
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 in main file, got %d", len(todos))
	}
	if todos[0].Name != "recurring task" {
		t.Fatalf("wrong item in main file: %q", todos[0].Name)
	}

	// Verify archive has the completed item
	archived, err := ReadTodos(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 in archive, got %d", len(archived))
	}
	if archived[0].Name != "one-off task" || archived[0].Status != StatusDone {
		t.Fatalf("archive item wrong: %q %q", archived[0].Name, archived[0].Status)
	}

	// Complete the recurring task
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	found, err = FindByID(todos, t2.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := CompleteTodo(found)
	if err := AppendTodo(archivePath, *found); err != nil {
		t.Fatal(err)
	}

	remaining = nil
	for _, item := range todos {
		if item.ID != found.ID {
			remaining = append(remaining, item)
		}
	}
	if next != nil {
		remaining = append(remaining, *next)
	}
	if err := WriteTodos(path, remaining); err != nil {
		t.Fatal(err)
	}

	// Main file should have only the new recurring item
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 in main after recurring done, got %d", len(todos))
	}
	if todos[0].Status != StatusTodo {
		t.Fatalf("new recurring item should be todo, got %q", todos[0].Status)
	}
	if todos[0].Recurrence == nil || *todos[0].Recurrence != "7d" {
		t.Fatal("new recurring item should keep recurrence")
	}

	// Archive should now have 2 items
	archived, err = ReadTodos(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 2 {
		t.Fatalf("expected 2 in archive, got %d", len(archived))
	}
}

func TestValidStatus(t *testing.T) {
	for _, s := range []string{"todo", "inprogress", "waiting", "done"} {
		if _, err := ValidStatus(s); err != nil {
			t.Errorf("expected %q to be valid", s)
		}
	}
	if _, err := ValidStatus("invalid"); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error("empty string should return nil")
	}
	p := strPtr("hello")
	if p == nil || *p != "hello" {
		t.Error("non-empty string should return pointer")
	}
}

func TestListOutputJSONL(t *testing.T) {
	t1 := NewTodo("first", strPtr("a description"), strPtr("work"), "2026-03-01", nil)
	t2 := NewTodo("second", nil, nil, "2026-03-02", nil)
	todos := []Todo{t1, t2}
	SortByDue(todos)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, td := range todos {
		if err := enc.Encode(td); err != nil {
			t.Fatal(err)
		}
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}

	// Decode each line and verify full ULID is present
	for i, line := range lines {
		var decoded Todo
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d: bad JSON: %v", i, err)
		}
		if len(decoded.ID) != 26 {
			t.Errorf("line %d: expected full 26-char ULID, got %d chars", i, len(decoded.ID))
		}
	}

	// Verify first line is t1 (sooner due)
	var first Todo
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Name != "first" {
		t.Errorf("expected first item 'first', got %q", first.Name)
	}
	if first.Description == nil || *first.Description != "a description" {
		t.Error("expected description to be present in JSON output")
	}
}

func TestFilterDateRange(t *testing.T) {
	t1 := NewTodo("early", nil, nil, "2026-02-10", nil)
	t2 := NewTodo("middle", nil, nil, "2026-02-20", nil)
	t3 := NewTodo("late", nil, nil, "2026-03-05", nil)
	todos := []Todo{t1, t2, t3}

	// --from only: items due on or after 2026-02-15
	from := "2026-02-15"
	filtered := FilterTodos(todos, false, nil, nil, false, &from, nil)
	if len(filtered) != 2 {
		t.Fatalf("--from: expected 2 items, got %d", len(filtered))
	}
	if filtered[0].Name != "middle" || filtered[1].Name != "late" {
		t.Fatalf("--from: wrong items: %q, %q", filtered[0].Name, filtered[1].Name)
	}

	// --to only: items due before 2026-02-25
	to := "2026-02-25"
	filtered = FilterTodos(todos, false, nil, nil, false, nil, &to)
	if len(filtered) != 2 {
		t.Fatalf("--to: expected 2 items, got %d", len(filtered))
	}
	if filtered[0].Name != "early" || filtered[1].Name != "middle" {
		t.Fatalf("--to: wrong items: %q, %q", filtered[0].Name, filtered[1].Name)
	}

	// --from and --to: items due in [2026-02-15, 2026-02-25)
	filtered = FilterTodos(todos, false, nil, nil, false, &from, &to)
	if len(filtered) != 1 {
		t.Fatalf("--from --to: expected 1 item, got %d", len(filtered))
	}
	if filtered[0].Name != "middle" {
		t.Fatalf("--from --to: expected 'middle', got %q", filtered[0].Name)
	}

	// --from at exact due date boundary (inclusive)
	exactFrom := "2026-02-20"
	filtered = FilterTodos(todos, false, nil, nil, false, &exactFrom, nil)
	if len(filtered) != 2 {
		t.Fatalf("--from exact: expected 2 items, got %d", len(filtered))
	}

	// --to at exact due date boundary (exclusive)
	exactTo := "2026-02-20"
	filtered = FilterTodos(todos, false, nil, nil, false, nil, &exactTo)
	if len(filtered) != 1 {
		t.Fatalf("--to exact: expected 1 item, got %d", len(filtered))
	}
	if filtered[0].Name != "early" {
		t.Fatalf("--to exact: expected 'early', got %q", filtered[0].Name)
	}

	// No range: all items returned
	filtered = FilterTodos(todos, false, nil, nil, false, nil, nil)
	if len(filtered) != 3 {
		t.Fatalf("no range: expected 3 items, got %d", len(filtered))
	}
}

func TestDueDateIsDateOnly(t *testing.T) {
	todo := NewTodo("test", nil, nil, "2026-03-15", nil)
	if todo.Due != "2026-03-15" {
		t.Fatalf("expected date-only due %q, got %q", "2026-03-15", todo.Due)
	}

	// Verify it round-trips through JSON without a time component
	b, err := json.Marshal(todo)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Todo
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Due != "2026-03-15" {
		t.Fatalf("expected date-only after round-trip, got %q", decoded.Due)
	}
}

func TestEndToEndWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e.jsonl")

	// Add
	t1 := NewTodo("buy groceries", strPtr("milk, eggs"), strPtr("personal"), "2026-03-01", nil)
	rec := "1w"
	t2 := NewTodo("weekly review", nil, strPtr("work"), "2026-03-02", &rec)
	if err := AppendTodo(path, t1); err != nil {
		t.Fatal(err)
	}
	if err := AppendTodo(path, t2); err != nil {
		t.Fatal(err)
	}

	// List
	todos, err := ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2, got %d", len(todos))
	}

	// Update
	found, err := FindByID(todos, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	found.Name = "buy organic groceries"
	for i, item := range todos {
		if item.ID == found.ID {
			todos[i] = *found
		}
	}
	if err := WriteTodos(path, todos); err != nil {
		t.Fatal(err)
	}

	// Done with recurrence
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	weekly, err := FindByID(todos, t2.ID)
	if err != nil {
		t.Fatal(err)
	}
	next := CompleteTodo(weekly)
	for i, item := range todos {
		if item.ID == weekly.ID {
			todos[i] = *weekly
		}
	}
	if next != nil {
		todos = append(todos, *next)
	}
	if err := WriteTodos(path, todos); err != nil {
		t.Fatal(err)
	}

	// Verify
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 3 {
		t.Fatalf("expected 3 after recurrence, got %d", len(todos))
	}

	doneCount := 0
	for _, item := range todos {
		if item.Status == StatusDone {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Fatalf("expected 1 done, got %d", doneCount)
	}

	// Remove
	var remaining []Todo
	for _, item := range todos {
		if item.ID != t1.ID {
			remaining = append(remaining, item)
		}
	}
	if err := WriteTodos(path, remaining); err != nil {
		t.Fatal(err)
	}
	todos, err = ReadTodos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 after rm, got %d", len(todos))
	}

	// Categories
	cats := Categories(todos)
	if len(cats) != 1 || cats[0] != "work" {
		t.Fatalf("expected [work], got %v", cats)
	}

	// Cleanup
	os.Remove(path)
}
