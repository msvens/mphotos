package dao

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"reflect"
	"strings"
	"unicode"
)

func lowerFirst(s string) string {
	copyStr := []rune(s)
	copyStr[0] = unicode.ToLower(copyStr[0])
	return string(copyStr)
}

func buildInsertNamed(table string, fields []string, ignore ...string) string {
	//builds a query of the form
	//"INSERT INTO table (f1, f2, f3) VALUES (:f1, :f2, :f3)
	var b strings.Builder
	if len(ignore) > 0 {
		fields = exclude(fields, ignore)
	}
	for _, v := range fields {
		fmt.Fprintf(&b, ":%s, ", v)
	}
	vals := b.String()
	vals = vals[:len(vals)-2]
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(fields, ", "), vals)
}

func buildUpdateNamed2(table string, fields []string, whereField string, ignore ...string) string {
	//builds a query of the form
	//UPDATE table SET f1=:f1...
	var b strings.Builder
	if len(ignore) > 0 {
		fields = exclude(fields, ignore)
	}
	for _, v := range fields {
		fmt.Fprintf(&b, "%s=:%s, ", v, v)
	}
	vals := b.String()
	vals = vals[:len(vals)-2]
	if whereField == "" {
		return fmt.Sprintf("UPDATE %s SET %s", table, vals)
	} else {
		return fmt.Sprintf("UPDATE %s SET %s WHERE %s=:%s", table, vals, whereField, whereField)
	}
}

func has(db *sqlx.DB, table string, whereCol string, check interface{}) bool {
	stmt := "SELECT 1 FROM " + table + " WHERE " + whereCol + " = $1"
	if rows, err := db.Query(stmt, check); err == nil {
		defer rows.Close()
		return rows.Next()
	} else {
		return false
	}
}

func exclude(strs []string, filter []string) []string {
	ret := []string{}
	for _, v := range strs {
		if !containsField(v, filter) {
			ret = append(ret, v)
		}
	}
	return ret
}

func containsField(str string, strs []string) bool {
	for _, v := range strs {
		if str == v {
			return true
		}
	}
	return false
}

func getStructFields(p interface{}) []string {
	val := reflect.Indirect(reflect.ValueOf(p))
	fields := make([]string, val.Type().NumField())
	for idx := 0; idx < len(fields); idx++ {
		//fields[idx] = lowerFirst(val.Type().Field(idx).Name)
		fields[idx] = strings.ToLower(val.Type().Field(idx).Name)
		//fields[idx] = val.Type().Field(idx).Name
	}
	return fields
}
