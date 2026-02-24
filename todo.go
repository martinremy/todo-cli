package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "inprogress"
	StatusWaiting    Status = "waiting"
	StatusDone       Status = "done"
)

var validStatuses = map[Status]bool{
	StatusTodo: true, StatusInProgress: true, StatusWaiting: true, StatusDone: true,
}

func ValidStatus(s string) (Status, error) {
	st := Status(s)
	if validStatuses[st] {
		return st, nil
	}
	return "", fmt.Errorf("invalid status %q (valid: todo, inprogress, waiting, done)", s)
}

type Todo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	Status      Status  `json:"status"`
	Created     string  `json:"created"`
	Updated     string  `json:"updated"`
	Due         string  `json:"due"`
	Recurrence  *string `json:"recurrence"`
}

func NewTodo(name string, desc, category *string, due string, recurrence *string) Todo {
	now := time.Now().UTC().Format(time.RFC3339)
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
	return Todo{
		ID:          id,
		Name:        name,
		Description: desc,
		Category:    category,
		Status:      StatusTodo,
		Created:     now,
		Updated:     now,
		Due:         due,
		Recurrence:  recurrence,
	}
}

// ReadTodos reads all todos from the JSONL file.
func ReadTodos(path string) ([]Todo, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var todos []Todo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var t Todo
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			return nil, fmt.Errorf("bad JSONL line: %w", err)
		}
		todos = append(todos, t)
	}
	return todos, scanner.Err()
}

// WriteTodos writes all todos to the JSONL file (full rewrite).
func WriteTodos(path string, todos []Todo) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, t := range todos {
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	return w.Flush()
}

// AppendTodo appends a single todo to the file.
func AppendTodo(path string, t Todo) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\n", b)
	return err
}

// FindByID finds a todo with an exact ID match.
func FindByID(todos []Todo, id string) (*Todo, error) {
	id = strings.ToUpper(id)
	for i := range todos {
		if todos[i].ID == id {
			return &todos[i], nil
		}
	}
	return nil, fmt.Errorf("no todo found with ID %q", id)
}

// SortByDue sorts todos by due date ascending.
func SortByDue(todos []Todo) {
	sort.Slice(todos, func(i, j int) bool {
		return todos[i].Due < todos[j].Due
	})
}

// FilterTodos returns a filtered list based on criteria.
func FilterTodos(todos []Todo, showAll bool, status *Status, category *string, overdue bool, from, to *string) []Todo {
	today := time.Now().UTC().Format("2006-01-02")
	var result []Todo
	for _, t := range todos {
		if !showAll && t.Status == StatusDone {
			continue
		}
		if status != nil && t.Status != *status {
			continue
		}
		if category != nil && (t.Category == nil || *t.Category != *category) {
			continue
		}
		if overdue {
			if t.Due >= today || t.Status == StatusDone {
				continue
			}
		}
		if from != nil && t.Due < *from {
			continue
		}
		if to != nil && t.Due >= *to {
			continue
		}
		result = append(result, t)
	}
	return result
}

// Categories returns distinct categories from the list.
func Categories(todos []Todo) []string {
	seen := map[string]bool{}
	for _, t := range todos {
		if t.Category != nil {
			seen[*t.Category] = true
		}
	}
	cats := make([]string, 0, len(seen))
	for c := range seen {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	return cats
}

// ParseDuration parses recurrence strings like "1d", "2w", "1m", "1y".
var recurrenceRe = regexp.MustCompile(`^(\d+)([dwmy])$`)

func ParseRecurrence(s string) (time.Duration, error) {
	m := recurrenceRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid recurrence %q (use e.g. 7d, 2w, 1m)", s)
	}
	n, _ := strconv.Atoi(m[1])
	switch m[2] {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("invalid recurrence unit in %q", s)
}

// CompleteTodo marks a todo as done and returns a new recurring todo if applicable.
func CompleteTodo(t *Todo) *Todo {
	now := time.Now().UTC()
	t.Status = StatusDone
	t.Updated = now.Format(time.RFC3339)

	if t.Recurrence == nil {
		return nil
	}

	dur, err := ParseRecurrence(*t.Recurrence)
	if err != nil {
		return nil
	}

	newDue := now.Add(dur).UTC().Format("2006-01-02")
	next := NewTodo(t.Name, t.Description, t.Category, newDue, t.Recurrence)
	return &next
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ArchivePath returns the archive file path for a given data file.
// e.g. ~/.todo.jsonl → ~/.todo.archive.jsonl
func ArchivePath(dataPath string) string {
	ext := filepath.Ext(dataPath)
	base := strings.TrimSuffix(dataPath, ext)
	return base + ".archive" + ext
}
