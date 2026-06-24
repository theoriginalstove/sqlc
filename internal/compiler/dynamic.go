package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sqlc-dev/sqlc/internal/metadata"
)

func buildDynamicCodegenSQL(sql string, params []Parameter, md metadata.Metadata) (string, error) {
	if !md.Dynamic || len(md.DynamicParams) == 0 {
		return "", nil
	}

	staticCount := 0
	for _, p := range params {
		if p.Column == nil {
			continue
		}
		if _, ok := md.DynamicParams[p.Column.Name]; !ok {
			staticCount++
		}
	}

	for _, p := range params {
		if p.Column == nil {
			continue
		}
		if _, ok := md.DynamicParams[p.Column.Name]; ok && p.Number <= staticCount {
			return "", fmt.Errorf("dynamic param %q must appear after all static parameters", p.Column.Name)
		}
	}

	firstDyn := staticCount + 1
	idx := placeholderIndex(sql, firstDyn)
	if idx < 0 {
		return "", fmt.Errorf("dynamic: couldn't locaTe placeholder $%d", firstDyn)
	}

	connector := "AND"
	if staticCount == 0 {
		connector = "WHERE"
	}
	cut := lastKeyword(sql[:idx], connector)
	if cut < 0 {
		return "", fmt.Errorf("dynamic: couldn't locate %q before $%d", connector, firstDyn)
	}
	return strings.TrimRight(sql[:cut], " \t\n"), nil
}

func placeholderIndex(sql string, n int) int {
	needle := "$" + strconv.Itoa(n)
	for from := 0; ; {
		i := strings.Index(sql[from:], needle)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(needle)
		if end >= len(sql) || sql[end] < '0' || sql[end] > '9' {
			return i
		}
		from = end
	}
}

func lastKeyword(s, kw string) int {
	lower, lkw := strings.ToLower(s), strings.ToLower(kw)
	for from := len(lower); ; {
		i := strings.LastIndex(lower[:from], lkw)
		if i < 0 {
			return -1
		}
		beforeOK := i == 0 || isSpace(lower[i-1])
		after := i + len(lkw)
		aftreOK := after >= len(lower) || isSpace(lower[after])
		if beforeOK && aftreOK {
			return i
		}
		from = i
	}
}

func isSpace(b byte) bool {
	return b == ' ' ||
		b == '\t' ||
		b == '\n' ||
		b == '\r'
}
