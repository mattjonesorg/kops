package main

import (
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"io"
	"k8s.io/kops/upup/pkg/fi"
	"reflect"
	"sort"
	"text/tabwriter"
)

// Table renders tables to stdout
type Table struct {
	columns map[string]*TableColumn
}

type TableColumn struct {
	Name   string
	Getter reflect.Value
}

func (c *TableColumn) getFromValue(v reflect.Value) string {
	var args []reflect.Value
	args = append(args, v)
	fvs := c.Getter.Call(args)
	fv := fvs[0]

	return fi.ValueAsString(fv)
}

type getterFunction func(interface{}) string

// AddColumn registers an available column for formatting
func (t *Table) AddColumn(name string, getter interface{}) {
	getterVal := reflect.ValueOf(getter)

	column := &TableColumn{
		Name:   name,
		Getter: getterVal,
	}
	if t.columns == nil {
		t.columns = make(map[string]*TableColumn)
	}
	t.columns[name] = column
}

type funcSorter struct {
	len  int
	less func(int, int) bool
	swap func(int, int)
}

func (f *funcSorter) Len() int {
	return f.len
}
func (f *funcSorter) Less(i, j int) bool {
	return f.less(i, j)
}
func (f *funcSorter) Swap(i, j int) {
	f.swap(i, j)
}

func SortByFunction(len int, swap func(int, int), less func(int, int) bool) {
	sort.Sort(&funcSorter{len, less, swap})
}

func (t *Table) findColumns(columnNames ...string) ([]*TableColumn, error) {
	columns := make([]*TableColumn, len(columnNames))
	for i, columnName := range columnNames {
		c := t.columns[columnName]
		if c == nil {
			return nil, fmt.Errorf("column not found: %v", columnName)
		}
		columns[i] = c
	}
	return columns, nil
}

// Render writes the items in a table, to out
func (t *Table) Render(items interface{}, out io.Writer, columnNames ...string) error {
	itemsValue := reflect.ValueOf(items)
	if itemsValue.Kind() != reflect.Slice {
		glog.Fatal("unexpected kind for items: ", itemsValue.Kind())
	}

	columns, err := t.findColumns(columnNames...)
	if err != nil {
		return err
	}

	n := itemsValue.Len()

	rows := make([][]string, n)
	for i := 0; i < n; i++ {
		row := make([]string, len(columns))
		item := itemsValue.Index(i)
		for j, column := range columns {
			row[j] = column.getFromValue(item)
		}
		rows[i] = row
	}

	SortByFunction(n, func(i, j int) {
		a := itemsValue.Index(i)
		itemsValue.Index(i).Set(itemsValue.Index(j))
		itemsValue.Index(j).Set(a)

		row := rows[i]
		rows[i] = rows[j]
		rows[j] = row
	}, func(i, j int) bool {
		l := rows[i]
		r := rows[j]

		for k := 0; k < len(columns); k++ {
			lV := l[k]
			rV := r[k]

			if lV != rV {
				return lV < rV
			}
		}
		return false
	})

	var b bytes.Buffer
	w := new(tabwriter.Writer)

	// Format in tab-separated columns with a tab stop of 8.
	w.Init(out, 0, 8, 0, '\t', tabwriter.StripEscape)

	writeHeader := true
	if writeHeader {
		for i, c := range columns {
			if i != 0 {
				b.WriteByte('\t')
			}
			b.WriteByte(tabwriter.Escape)
			b.WriteString(c.Name)
			b.WriteByte(tabwriter.Escape)
		}
		b.WriteByte('\n')

		_, err := w.Write(b.Bytes())
		if err != nil {
			return fmt.Errorf("error writing to output: %v", err)
		}
		b.Reset()
	}

	for _, row := range rows {
		for i, col := range row {
			if i != 0 {
				b.WriteByte('\t')
			}

			b.WriteByte(tabwriter.Escape)
			b.WriteString(col)
			b.WriteByte(tabwriter.Escape)
		}
		b.WriteByte('\n')

		_, err := w.Write(b.Bytes())
		if err != nil {
			return fmt.Errorf("error writing to output: %v", err)
		}
		b.Reset()
	}
	w.Flush()

	return nil
}
