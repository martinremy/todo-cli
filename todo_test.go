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
	todo := NewTodo("test item", nil, nil, time.Now().Add(14*24*time.Hour), nil)
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
}

func TestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	t1 := NewTodo("first", strPtr("desc1"), strPtr("work"), time.Now(), nil)
	t2 := NewTodo("second", nil, nil, time.Now(), nil)

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
	t1 := NewTodo("item1", nil, nil, time.Now(), nil)
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
	t1 := NewTodo("active", nil, nil, time.Now(), nil)
	t2 := NewTodo("finished", nil, nil, time.Now(), nil)
	t2.Status = StatusDone

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, nil, false)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-done item, got %d", len(filtered))
	}
	if filtered[0].Name != "active" {
		t.Fatal("wrong item")
	}
}

func TestFilterAll(t *testing.T) {
	t1 := NewTodo("active", nil, nil, time.Now(), nil)
	t2 := NewTodo("finished", nil, nil, time.Now(), nil)
	t2.Status = StatusDone

	filtered := FilterTodos([]Todo{t1, t2}, true, nil, nil, false)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items with --all, got %d", len(filtered))
	}
}

func TestListAllIncludesArchived(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	archivePath := ArchivePath(path)

	active := NewTodo("active task", nil, strPtr("work"), time.Now().Add(24*time.Hour), nil)
	done := NewTodo("done task", nil, strPtr("work"), time.Now().Add(48*time.Hour), nil)
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
	filtered := FilterTodos(activeTodos, false, nil, nil, false)
	if len(filtered) != 1 || filtered[0].Name != "active task" {
		t.Fatalf("expected only active task, got %d items", len(filtered))
	}

	// With archive (simulating --all): both items
	archived, err := ReadTodos(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	all := append(activeTodos, archived...)
	filteredAll := FilterTodos(all, true, nil, nil, false)
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
	t1 := NewTodo("item1", nil, &cat, time.Now(), nil)
	t2 := NewTodo("item2", nil, nil, time.Now(), nil)

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, &cat, false)
	if len(filtered) != 1 || filtered[0].Name != "item1" {
		t.Fatal("category filter failed")
	}
}

func TestFilterOverdue(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)
	t1 := NewTodo("overdue", nil, nil, past, nil)
	t2 := NewTodo("upcoming", nil, nil, future, nil)

	filtered := FilterTodos([]Todo{t1, t2}, false, nil, nil, true)
	if len(filtered) != 1 || filtered[0].Name != "overdue" {
		t.Fatal("overdue filter failed")
	}
}

func TestCompleteTodoNoRecurrence(t *testing.T) {
	todo := NewTodo("one-off", nil, nil, time.Now(), nil)
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
	todo := NewTodo("weekly task", nil, strPtr("chores"), time.Now(), &rec)
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

	nextDue, err := time.Parse(time.RFC3339, next.Due)
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
		NewTodo("a", nil, &c1, time.Now(), nil),
		NewTodo("b", nil, &c2, time.Now(), nil),
		NewTodo("c", nil, &c1, time.Now(), nil),
		NewTodo("d", nil, nil, time.Now(), nil),
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
	t1 := NewTodo("later", nil, nil, time.Now().Add(48*time.Hour), nil)
	t2 := NewTodo("sooner", nil, nil, time.Now().Add(24*time.Hour), nil)
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
		NewTodo("a", nil, nil, time.Now(), nil),
		NewTodo("b", nil, nil, time.Now(), nil),
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
	t1 := NewTodo("one-off task", nil, strPtr("work"), time.Now().Add(24*time.Hour), nil)
	t2 := NewTodo("recurring task", nil, strPtr("work"), time.Now().Add(48*time.Hour), &rec)
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
	t1 := NewTodo("first", strPtr("a description"), strPtr("work"), time.Now().Add(24*time.Hour), nil)
	t2 := NewTodo("second", nil, nil, time.Now().Add(48*time.Hour), nil)
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

func TestEndToEndWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e.jsonl")

	// Add
	t1 := NewTodo("buy groceries", strPtr("milk, eggs"), strPtr("personal"), time.Now().Add(24*time.Hour), nil)
	rec := "1w"
	t2 := NewTodo("weekly review", nil, strPtr("work"), time.Now().Add(48*time.Hour), &rec)
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
