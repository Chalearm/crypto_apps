/******************************************************************************
 * File Name       : comment_checker.go
 * File Path       : regulator/comment_checker.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-26 12:00:00 (UTC+7)
 * Modified Date   : 2026-07-26 12:00:00 (UTC+7)
 *
 * Description     :
 *   Validates function doc-comment headers across Go, JavaScript, and Python
 *   files to ensure adherence to required standard sections.
 *
 * Responsibilities:
 *   - Walk project files matching supported extensions (.go, .js, .py).
 *   - Parse function signatures and verify existence of doc comment blocks.
 *   - Ensure required section headers and function names match signatures.
 *
 * Usage :
 *   Directory : regulator/
 *
 * Dependencies :
 *   Internal :
 *     - None
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   [Function]
 *     - None
 *
 * New Parts :
 *   [Function]
 *     - CheckFunctionDocComments()
 *     - scanFileForDocComments()
 *     - extractFunctionName()
 *     - extractPrecedingCommentBlock()
 *     - checkMissingSections()
 *     - PrintViolationTable()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 12:00:00      | Gemini 3.1 Pro  | Initial doc linter
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Support C/C++ function signature doc checking.
 *
 * Notes :
 *   - Test files (*_test.go) are skipped.
 ******************************************************************************/

package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
)

var (
	goFuncRegex = regexp.MustCompile(`^\s*func\s+([A-Za-z0-9_]+)\s*\(`)
	jsFuncRegex = regexp.MustCompile(`^\s*(?:async\s+)?function\s+([A-Za-z0-9_]+)\s*\(|^\s*(?:const|let|var)\s+([A-Za-z0-9_]+)\s*=\s*(?:async\s*)?\(`)
	pyFuncRegex = regexp.MustCompile(`^\s*def\s+([A-Za-z0-9_]+)\s*\(`)
)

var requiredDocSections = []string{
	"Function Name",
	"Purpose",
	"Inputs",
	"Return",
	"Complexity",
	"Error Cases",
	"Number Of Lines",
}

type CommentViolation struct {
	FilePath     string
	LineNumber   int
	FunctionName string
	Reason       string
}

/******************************************************************************
 * Function Name : CheckFunctionDocComments
 *
 * Purpose :
 *   Walks target directory to check function doc comment compliance.
 *
 * Inputs :
 *   targetDir
 *     Type        : string
 *     Description : Directory pathway to evaluate.
 *
 * Return :
 *   Type        : []CommentViolation, error
 *   Description : List of violations detected across all functions.
 *
 * Complexity :
 *   Time  : O(F * L)
 *   Space : O(V)
 *
 * Error Cases :
 *   - Returns error on directory walk failures.
 *
 * Number Of Lines :
 *   35
 ******************************************************************************/
func CheckFunctionDocComments(targetDir string) ([]CommentViolation, error) {
	var violations []CommentViolation

	err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skippedDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".go" && ext != ".js" && ext != ".py" {
			return nil
		}

		if strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		fileViolations, err := scanFileForDocComments(path, ext)
		if err != nil {
			return nil
		}

		violations = append(violations, fileViolations...)
		return nil
	})

	return violations, err
}

/******************************************************************************
 * Function Name : scanFileForDocComments
 *
 * Purpose :
 *   Inspects a file line-by-line to check each function's doc comment block.
 *
 * Inputs :
 *   filePath, ext
 *     Type        : string
 *     Description : Path of file and file extension.
 *
 * Return :
 *   Type        : []CommentViolation, error
 *   Description : Doc comment violations found in file.
 *
 * Complexity :
 *   Time  : O(N) where N is line count.
 *   Space : O(N)
 *
 * Error Cases :
 *   - File open errors returned.
 *
 * Number Of Lines :
 *   50
 ******************************************************************************/
func scanFileForDocComments(filePath string, ext string) ([]CommentViolation, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var violations []CommentViolation

	for i, line := range lines {
		funcName := extractFunctionName(line, ext)
		if funcName == "" {
			continue
		}

		lineNumber := i + 1

		docBlock := extractPrecedingCommentBlock(lines, i)
		if docBlock == "" {
			violations = append(violations, CommentViolation{
				FilePath:     filePath,
				LineNumber:   lineNumber,
				FunctionName: funcName,
				Reason:       "Missing doc comment block entirely",
			})
			continue
		}

		missingSections := checkMissingSections(docBlock)
		if len(missingSections) > 0 {
			violations = append(violations, CommentViolation{
				FilePath:     filePath,
				LineNumber:   lineNumber,
				FunctionName: funcName,
				Reason:       fmt.Sprintf("Missing section(s): %s", strings.Join(missingSections, ", ")),
			})
			continue
		}

		if !strings.Contains(docBlock, "Function Name : "+funcName) {
			violations = append(violations, CommentViolation{
				FilePath:     filePath,
				LineNumber:   lineNumber,
				FunctionName: funcName,
				Reason:       fmt.Sprintf("Doc comment 'Function Name' does not match '%s'", funcName),
			})
		}
	}

	return violations, scanner.Err()
}

/******************************************************************************
 * Function Name : extractFunctionName
 *
 * Purpose :
 *   Extracts declared function identifier using language-specific regexes.
 *
 * Inputs :
 *   line, ext
 *     Type        : string
 *     Description : Source line and file extension.
 *
 * Return :
 *   Type        : string
 *   Description : Function name if line contains definition.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - Returns empty string if line is not a function definition.
 *
 * Number Of Lines :
 *   20
 ******************************************************************************/
func extractFunctionName(line string, ext string) string {
	var matches []string
	switch ext {
	case ".go":
		matches = goFuncRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	case ".js":
		matches = jsFuncRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			if matches[1] != "" {
				return matches[1]
			}
			return matches[2]
		}
	case ".py":
		matches = pyFuncRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

/******************************************************************************
 * Function Name : extractPrecedingCommentBlock
 *
 * Purpose :
 *   Reads backward from a function line to capture the preceding comment block.
 *
 * Inputs :
 *   lines, funcLineIdx
 *     Type        : []string, int
 *     Description : Entire file line slice and index of function.
 *
 * Return :
 *   Type        : string
 *   Description : Captured comment block string.
 *
 * Complexity :
 *   Time  : O(C) where C is comment block length.
 *   Space : O(C)
 *
 * Error Cases :
 *   - Returns empty string if no comment precedes function.
 *
 * Number Of Lines :
 *   22
 ******************************************************************************/
func extractPrecedingCommentBlock(lines []string, funcLineIdx int) string {
	var block []string
	idx := funcLineIdx - 1

	for idx >= 0 && strings.TrimSpace(lines[idx]) == "" {
		idx--
	}

	for idx >= 0 {
		trimmed := strings.TrimSpace(lines[idx])
		if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") || strings.HasSuffix(trimmed, "*/") || strings.HasPrefix(trimmed, "#") {
			block = append([]string{lines[idx]}, block...)
			if strings.HasPrefix(trimmed, "/*") {
				break
			}
			idx--
		} else {
			break
		}
	}

	return strings.Join(block, "\n")
}

/******************************************************************************
 * Function Name : checkMissingSections
 *
 * Purpose :
 *   Checks doc block against required section titles.
 *
 * Inputs :
 *   docBlock
 *     Type        : string
 *     Description : Captured comment block text.
 *
 * Return :
 *   Type        : []string
 *   Description : Missing required section names.
 *
 * Complexity :
 *   Time  : O(S) where S is number of required sections.
 *   Space : O(M) where M is missing section count.
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func checkMissingSections(docBlock string) []string {
	var missing []string
	for _, sec := range requiredDocSections {
		if !strings.Contains(docBlock, sec) {
			missing = append(missing, sec)
		}
	}
	return missing
}

/******************************************************************************
 * Function Name : PrintViolationTable
 *
 * Purpose :
 *   Formats and displays function doc violations as a CLI table.
 *
 * Inputs :
 *   violations
 *     Type        : []CommentViolation
 *     Description : Slice of detected violations.
 *
 * Return :
 *   Type        : void
 *   Description : Outputs directly to stdout.
 *
 * Complexity :
 *   Time  : O(V)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   12
 ******************************************************************************/
func PrintViolationTable(violations []CommentViolation) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "FILE PATH\tLINE\tFUNCTION NAME\tREASON")
	fmt.Fprintln(w, "---------\t----\t-------------\t------")

	for _, v := range violations {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", v.FilePath, v.LineNumber, v.FunctionName, v.Reason)
	}
	w.Flush()
}