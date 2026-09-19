package schema

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	_ "modernc.org/sqlite"
)

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

type Snapshot struct {
	Tables      []Table `json:"tables"`
	Fingerprint string  `json:"fingerprint"`
}

func Inspect(dbPath string) (Snapshot, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()

	var snap Snapshot
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return Snapshot{}, err
		}
		cols, err := tableColumns(db, name)
		if err != nil {
			return Snapshot{}, err
		}
		snap.Tables = append(snap.Tables, Table{Name: name, Columns: cols})
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, err
	}
	snap.Fingerprint = fingerprint(snap.Tables)
	return snap, nil
}

func tableColumns(db *sql.DB, table string) ([]Column, error) {
	rows, err := db.Query(`PRAGMA table_info("` + table + `")`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, Column{Name: name, Type: typ})
	}
	return cols, rows.Err()
}

func fingerprint(tables []Table) string {
	type colSig struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type tabSig struct {
		Name    string   `json:"name"`
		Columns []colSig `json:"columns"`
	}
	sigs := make([]tabSig, 0, len(tables))
	for _, t := range tables {
		cs := make([]colSig, len(t.Columns))
		for i, c := range t.Columns {
			cs[i] = colSig{Name: c.Name, Type: c.Type}
		}
		sigs = append(sigs, tabSig{Name: t.Name, Columns: cs})
	}
	sort.Slice(sigs, func(i, j int) bool { return sigs[i].Name < sigs[j].Name })
	data, _ := json.Marshal(sigs)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func LoadExpected(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, err
	}
	if snap.Fingerprint == "" {
		snap.Fingerprint = fingerprint(snap.Tables)
	}
	return snap, nil
}

func Save(path string, snap Snapshot) error {
	snap.Fingerprint = fingerprint(snap.Tables)
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Check(dbPath, expectedPath string) error {
	got, err := Inspect(dbPath)
	if err != nil {
		return err
	}
	want, err := LoadExpected(expectedPath)
	if err != nil {
		return fmt.Errorf("load expected schema: %w", err)
	}
	if got.Fingerprint != want.Fingerprint {
		return fmt.Errorf("schema fingerprint mismatch: got %s want %s (run: bible schema refresh)", got.Fingerprint, want.Fingerprint)
	}
	return nil
}

func Refresh(dbPath, expectedPath string) error {
	snap, err := Inspect(dbPath)
	if err != nil {
		return err
	}
	return Save(expectedPath, snap)
}
