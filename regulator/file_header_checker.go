/******************************************************************************
 * File Name       : file_header_checker.go
 * File Path       : regulator/file_header_checker.go
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
 *   Validates top-level file header comments across multi-language source files
 *   (.go, .py, .sh, .c, .h, .cpp, .hpp, .js, .html).
 *
 * Responsibilities:
 *   - Normalize language-specific comment characters (#, //, /*, <!--).
 *   - Verify presence of required top-level file header blocks.
 *   - Verify declared File Name matches actual file name on disk.
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
 *     - CheckFileHeaders()
 *     - scanFileHeader()
 *     - extractAndNormalizeHeader()
 *     - checkMissingHeaderSections()
 *     - isSupportedFileExtension()
 *     - PrintHeaderViolationTable()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 12:00:00      | Gemini 3.1 Pro  | Initial header linter
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Support custom rule files.
 *
 * Notes :
 *   - Handles shebang lines and doctype headers gracefully.
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

var requiredFileHeaderSections = []string{
	"File Name",
	"File Path",
	"Author",
	"Owner",
	"Reviewer",
	"Version",
	"Status",
	"Created Date",
	"Modified Date",
	"Description",
	"Responsibilities",
	"Usage",
	"Dependencies",
	"Updated Parts",
	"New Parts",
	"Change History",
	"TODO",
	"Notes",
}

type HeaderViolation struct {
	FilePath string
	FileName string
	Reason   string
}

/******************************************************************************
 * Function Name : CheckFileHeaders
 *
 * Purpose :
 *   Walks the directory structure and audits file header compliance.
 *
 * Inputs :
 *   targetDir
 *     Type        : string
 *     Description : Pathway of directory to walk.
 *
 * Return :
 *   Type        : []HeaderViolation, error
 *   Description : Header violations discovered.
 *
 * Complexity :
 *   Time  : O(F)
 *   Space : O(V)
 *
 * Error Cases :
 *   - Returns error on directory walk failure.
 *
 * Number Of Lines :
 *   35
 ******************************************************************************/
func CheckFileHeaders(targetDir string) ([]HeaderViolation, error) {
	var violations []HeaderViolation

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
		if !isSupportedFileExtension(ext) {
			return nil
		}

		if strings.HasSuffix(d.Name(), "_test.go") || isBinaryExtension(d.Name()) {
			return nil
		}

		fileViolations, err := scanFileHeader(path)
		if err != nil {
			return nil
		}

		violations = append(violations, fileViolations...)
		return nil
	})

	return violations, err
}

/******************************************************************************
 * Function Name : scanFileHeader
 *
 * Purpose :
 *   Reads the top lines of a file and verifies its normalized header block.
 *
 * Inputs :
 *   filePath
 *     Type        : string
 *     Description : Path of file to audit.
 *
 * Return :
 *   Type        : []HeaderViolation, error
 *   Description : Discovered violations in target file.
 *
 * Complexity :
 *   Time  : O(1) - Reads max 120 lines.
 *   Space : O(1)
 *
 * Error Cases :
 *   - Returns error if file cannot be opened.
 *
 * Number Of Lines :
 *   50
 ******************************************************************************/
func scanFileHeader(filePath string) ([]HeaderViolation, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	count := 0
	for scanner.Scan() && count < 120 {
		lines = append(lines, scanner.Text())
		count++
	}

	actualFileName := filepath.Base(filePath)
	normalizedHeader := extractAndNormalizeHeader(lines)

	var violations []HeaderViolation

	if normalizedHeader == "" {
		return []HeaderViolation{{
			FilePath: filePath,
			FileName: actualFileName,
			Reason:   "Missing top-level file header comment block entirely",
		}}, nil
	}

	// 1. Check required header sections
	missingSections := checkMissingHeaderSections(normalizedHeader)
	if len(missingSections) > 0 {
		violations = append(violations, HeaderViolation{
			FilePath: filePath,
			FileName: actualFileName,
			Reason:   fmt.Sprintf("Missing header section(s): %s", strings.Join(missingSections, ", ")),
		})
	}

	// 2. Validate declared File Name matches actual filename on disk (flexible spacing)
	fileNamePattern := fmt.Sprintf(`(?i)File\s+Name\s*:\s*%s\b`, regexp.QuoteMeta(actualFileName))
	matchedFileName, _ := regexp.MatchString(fileNamePattern, normalizedHeader)
	if !matchedFileName {
		violations = append(violations, HeaderViolation{
			FilePath: filePath,
			FileName: actualFileName,
			Reason:   fmt.Sprintf("Header 'File Name' declaration does not match actual filename '%s'", actualFileName),
		})
	}

	return violations, scanner.Err()
}

/******************************************************************************
 * Function Name : extractAndNormalizeHeader
 *
 * Purpose :
 *   Strips language-specific comment characters to yield standardized text.
 *
 * Inputs :
 *   lines
 *     Type        : []string
 *     Description : Lines read from top of file.
 *
 * Return :
 *   Type        : string
 *   Description : Normalized header block string.
 *
 * Complexity :
 *   Time  : O(L) where L is line count.
 *   Space : O(L)
 *
 * Error Cases :
 *   - Returns empty string if no valid header comment block exists at top.
 *
 * Number Of Lines :
 *   40
 ******************************************************************************/
func extractAndNormalizeHeader(lines []string) string {
	var cleanLines []string
	inHeader := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#!") || strings.HasPrefix(strings.ToLower(trimmed), "<!doctype") {
			continue
		}

		if strings.Contains(trimmed, "*****") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			inHeader = true
		}

		if !inHeader && trimmed == "" {
			continue
		}

		if !inHeader {
			break
		}

		clean := trimmed
		clean = regexp.MustCompile(`^/\*+`).ReplaceAllString(clean, "")
		clean = regexp.MustCompile(`\*+/+$`).ReplaceAllString(clean, "")
		clean = regexp.MustCompile(`^<!--+`).ReplaceAllString(clean, "")
		clean = regexp.MustCompile(`--+>$`).ReplaceAllString(clean, "")
		clean = regexp.MustCompile(`^[#/\*]+`).ReplaceAllString(clean, "")
		clean = strings.TrimSpace(clean)

		cleanLines = append(cleanLines, clean)

		if strings.HasSuffix(trimmed, "*/") || strings.HasSuffix(trimmed, "-->") {
			break
		}
	}

	return strings.Join(cleanLines, "\n")
}

/******************************************************************************
 * Function Name : checkMissingHeaderSections
 *
 * Purpose :
 *   Checks normalized header for required section titles.
 *
 * Inputs :
 *   header
 *     Type        : string
 *     Description : Normalized header string.
 *
 * Return :
 *   Type        : []string
 *   Description : Slice of missing required section names.
 *
 * Complexity :
 *   Time  : O(S)
 *   Space : O(M)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   12
 ******************************************************************************/
func checkMissingHeaderSections(header string) []string {
	var missing []string
	for _, sec := range requiredFileHeaderSections {
		pattern := fmt.Sprintf(`(?i)%s\s*:`, regexp.QuoteMeta(sec))
		matched, _ := regexp.MatchString(pattern, header)
		if !matched {
			missing = append(missing, sec)
		}
	}
	return missing
}

/******************************************************************************
 * Function Name : isSupportedFileExtension
 *
 * Purpose :
 *   Checks whether file extension is audited for file headers.
 *
 * Inputs :
 *   ext
 *     Type        : string
 *     Description : Extension string.
 *
 * Return :
 *   Type        : bool
 *   Description : True if supported extension.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func isSupportedFileExtension(ext string) bool {
	switch ext {
	case ".go", ".py", ".sh", ".c", ".h", ".cpp", ".hpp", ".js", ".html":
		return true
	default:
		return false
	}
}

/******************************************************************************
 * Function Name : PrintHeaderViolationTable
 *
 * Purpose :
 *   Formats and displays file header violations as a CLI table.
 *
 * Inputs :
 *   violations
 *     Type        : []HeaderViolation
 *     Description : Violations to display.
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
func PrintHeaderViolationTable(violations []HeaderViolation) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "FILE PATH\tFILE NAME\tREASON")
	fmt.Fprintln(w, "---------\t---------\t------")

	for _, v := range violations {
		fmt.Fprintf(w, "%s\t%s\t%s\n", v.FilePath, v.FileName, v.Reason)
	}
	w.Flush()
}