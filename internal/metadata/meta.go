package metadata

import (
	"bufio"
	"fmt"
	"strings"
	"unicode"

	"github.com/sqlc-dev/sqlc/internal/constants"

	"github.com/sqlc-dev/sqlc/internal/source"
)

type CommentSyntax source.CommentSyntax

type Metadata struct {
	Name     string
	Cmd      string
	Comments []string
	Params   map[string]string
	Flags    map[string]bool

	// RuleSkiplist contains the names of rules to disable vetting for.
	// If the map is empty, but the disable vet flag is specified, then all rules are ignored.
	RuleSkiplist map[string]struct{}

	Filename string

	// Dynamic is set when the query is marked `:dynamic`.
	Dynamic bool
	// DynamicParams is the set of sqlc.arg names marked `@dynamic`
	DynamicParams map[string]string
	// DynamicSort is the whitelist of columns allowed in a dynamic ORDR BY clause.
	DynamicSort []string
}

const (
	CmdExec       = ":exec"
	CmdExecResult = ":execresult"
	CmdExecRows   = ":execrows"
	CmdExecLastId = ":execlastid"
	CmdMany       = ":many"
	CmdOne        = ":one"
	CmdCopyFrom   = ":copyfrom"
	CmdBatchExec  = ":batchexec"
	CmdBatchMany  = ":batchmany"
	CmdBatchOne   = ":batchone"
)

// A query name must be a valid Go identifier
//
// https://golang.org/ref/spec#Identifiers
func validateQueryName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("invalid query name: %q", name)
	}
	for i, c := range name {
		isLetter := unicode.IsLetter(c) || c == '_'
		isDigit := unicode.IsDigit(c)
		if i == 0 && !isLetter {
			return fmt.Errorf("invalid query name %q", name)
		} else if !(isLetter || isDigit) {
			return fmt.Errorf("invalid query name %q", name)
		}
	}
	return nil
}

func ParseQueryNameAndType(t string, commentStyle CommentSyntax) (string, string, bool, error) {
	for line := range strings.SplitSeq(t, "\n") {
		var prefix string
		if strings.HasPrefix(line, "--") {
			if !commentStyle.Dash {
				continue
			}
			prefix = "--"
		}
		if strings.HasPrefix(line, "/*") {
			if !commentStyle.SlashStar {
				continue
			}
			prefix = "/*"
		}
		if strings.HasPrefix(line, "#") {
			if !commentStyle.Hash {
				continue
			}
			prefix = "#"
		}
		if prefix == "" {
			continue
		}
		rest := line[len(prefix):]
		if !strings.HasPrefix(strings.TrimSpace(rest), "name") {
			continue
		}
		if !strings.Contains(rest, ":") {
			continue
		}
		if !strings.HasPrefix(rest, " name: ") {
			return "", "", false, fmt.Errorf("invalid metadata: %s", line)
		}

		part := strings.Split(strings.TrimSpace(line), " ")
		if prefix == "/*" {
			part = part[:len(part)-1] // removes the trailing "*/" element
		}
		if len(part) < 3 {
			return "", "", false, fmt.Errorf("invalid query comment: %s", line)
		}
		if len(part) == 3 {
			return "", "", false, fmt.Errorf("missing query type [':one', ':many', ':exec', ':execrows', ':execlastid', ':execresult', ':copyfrom', 'batchexec', 'batchmany', 'batchone']: %s", line)
		}
		queryName := part[2]
		queryType := strings.TrimSpace(part[3])
		switch queryType {
		case CmdOne, CmdMany, CmdExec, CmdExecResult, CmdExecRows, CmdExecLastId, CmdCopyFrom, CmdBatchExec, CmdBatchMany, CmdBatchOne:
		default:
			return "", "", false, fmt.Errorf("invalid query type: %s", queryType)
		}
		if err := validateQueryName(queryName); err != nil {
			return "", "", false, err
		}
		var dynamic bool
		for _, modifier := range part[4:] {
			switch strings.TrimSpace(modifier) {
			case ":dynamic":
				dynamic = true
			default:
				return "", "", false, fmt.Errorf("invalid query modifier %q: %s", modifier, line)
			}
		}
		return queryName, queryType, dynamic, nil
	}
	return "", "", false, nil
}

// ParseCommentFlags processes the comments provided with queries to determine the metadata params, flags and rules to skip.
// All flags in query comments are prefixed with `@`, e.g. @param, @@sqlc-vet-disable.
func ParseCommentFlags(comments []string) (map[string]string, map[string]bool, map[string]struct{}, map[string]string, []string, error) {
	params := make(map[string]string)
	flags := make(map[string]bool)
	ruleSkiplist := make(map[string]struct{})
	dynamicParams := make(map[string]string)
	dynamicSort := []string{}

	for _, line := range comments {
		s := bufio.NewScanner(strings.NewReader(line))
		s.Split(bufio.ScanWords)

		s.Scan()
		token := s.Text()

		if !strings.HasPrefix(token, "@") {
			continue
		}

		switch token {
		case constants.QueryFlagParam:
			s.Scan()
			name := s.Text()
			var rest []string
			for s.Scan() {
				paramToken := s.Text()
				rest = append(rest, paramToken)
			}
			params[name] = strings.Join(rest, " ")

		case constants.QueryFlagSqlcVetDisable:
			flags[token] = true

			// Vet rules can all be disabled in the same line or split across lines .i.e.
			// /* @sqlc-vet-disable sqlc/db-prepare delete-without-where */
			// is equivalent to:
			// /* @sqlc-vet-disable sqlc/db-prepare */
			// /* @sqlc-vet-disable delete-without-where */
			for s.Scan() {
				ruleSkiplist[s.Text()] = struct{}{}
			}
		case constants.QueryFlagDynamic:
			s.Scan()
			name := s.Text()
			s.Scan()
			op := s.Text()
			if name != "" {
				dynamicParams[name] = op
			}
		case constants.QueryFlagDynamicSort:
			for s.Scan() {
				col := strings.TrimSuffix(s.Text(), ",")
				if col != "" {
					dynamicSort = append(dynamicSort, col)
				}
			}

		default:
			flags[token] = true
		}

		if s.Err() != nil {
			return params, flags, ruleSkiplist, dynamicParams, dynamicSort, s.Err()
		}
	}

	return params, flags, ruleSkiplist, dynamicParams, dynamicSort, nil
}
