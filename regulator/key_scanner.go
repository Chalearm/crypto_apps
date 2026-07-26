/******************************************************************************
 * File Name       : key_scanner.go
 * File Path       : regulator/key_scanner.go
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
 *   Core scanning engine for detecting unauthorized private keys and high-entropy
 *   secret key snippets using regex patterns and Levenshtein similarity.
 *
 * Responsibilities:
 *   - Walk file trees while skipping binary files and allowed config files.
 *   - Search for full 64-character hex private keys and snippet prefixes.
 *   - Calculate Levenshtein distance similarity against reference key.
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
 *     - RunScan()
 *     - scanFileForKeys()
 *     - calculateSimilarity()
 *     - levenshteinDistance()
 *     - min()
 *     - isZeroOrDummyKey()
 *     - isBinaryExtension()
 *     - loadConfigEnv()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 12:00:00      | Gemini 3.1 Pro  | Initial engine
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add entropy threshold checking for non-hex keys.
 *
 * Notes :
 *   - Config env files are treated as allowed exceptions.
 ******************************************************************************/

package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ReferenceKey        = ""
	SimilarityThreshold = 20.5

	namedKeyRegex  = regexp.MustCompile(`(?i)(PRIVATE_?KEY|SECRET_?KEY|ETH_?KEY|WALLET_?KEY)\s*[:=]\s*["']?(0x)?[a-fA-F0-9]{64}["']?`)
	rawHexKeyRegex = regexp.MustCompile(`(?i)\b(0x)?[a-fA-F0-9]{64}\b`)
)

var allowedFileNames = map[string]bool{
	"config.env":  true,
	"config.json": true,
}

var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"venv":         true,
	".venv":        true,
	"env":          true,
	"build":        true,
	"dist":         true,
}

type KeyMatch struct {
	FilePath   string
	LineNumber int
	LineText   string
	MatchedKey string
	Similarity float64
}

/******************************************************************************
 * Function Name : loadConfigEnv
 *
 * Purpose :
 *   Parses key-value pairs from a target env file into system environment variables.
 *
 * Inputs :
 *   filename
 *     Type        : string
 *     Description : Path to target environment file.
 *
 * Return :
 *   Type        : void
 *   Description : Sets process environment variables on success.
 *
 * Complexity :
 *   Time  : O(N) where N is line count.
 *   Space : O(1)
 *
 * Error Cases :
 *   - File read errors are ignored gracefully.
 *
 * Number Of Lines :
 *   20
 ******************************************************************************/
func loadConfigEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			os.Setenv(key, val)
		}
	}
}

/******************************************************************************
 * Function Name : RunScan
 *
 * Purpose :
 *   Recursively walks the target directory to scan all non-binary files for keys.
 *
 * Inputs :
 *   targetDir
 *     Type        : string
 *     Description : Directory pathway to walk.
 *
 * Return :
 *   Type        : []KeyMatch, error
 *   Description : Slice of detected key exposure violations.
 *
 * Complexity :
 *   Time  : O(F * L) where F is file count and L is average lines per file.
 *   Space : O(V) where V is violation count.
 *
 * Error Cases :
 *   - File system walk errors returned directly.
 *
 * Number Of Lines :
 *   30
 ******************************************************************************/
func RunScan(targetDir string) ([]KeyMatch, error) {
	var violations []KeyMatch

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

		fileName := strings.ToLower(d.Name())

		if allowedFileNames[fileName] || isBinaryExtension(fileName) {
			return nil
		}

		matches, err := scanFileForKeys(path)
		if err != nil {
			return nil
		}

		violations = append(violations, matches...)
		return nil
	})

	return violations, err
}

/******************************************************************************
 * Function Name : scanFileForKeys
 *
 * Purpose :
 *   Scans an individual file line-by-line for raw hex key patterns and snippets.
 *
 * Inputs :
 *   filePath
 *     Type        : string
 *     Description : Full path of the file to audit.
 *
 * Return :
 *   Type        : []KeyMatch, error
 *   Description : Matches found in the specified file.
 *
 * Complexity :
 *   Time  : O(N) where N is line count.
 *   Space : O(M) where M is matches count.
 *
 * Error Cases :
 *   - Returns error if file cannot be opened.
 *
 * Number Of Lines :
 *   60
 ******************************************************************************/
func scanFileForKeys(filePath string) ([]KeyMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []KeyMatch
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	refClean := strings.ToLower(strings.TrimPrefix(ReferenceKey, "0x"))
	prefixSnippet := ""
	if len(refClean) >= 4 {
		prefixSnippet = refClean[:4]
	}

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		lowerLine := strings.ToLower(line)

		if prefixSnippet != "" && strings.Contains(lowerLine, prefixSnippet) {
			matches = append(matches, KeyMatch{
				FilePath:   filePath,
				LineNumber: lineNumber,
				LineText:   line,
				MatchedKey: fmt.Sprintf("Substring match: '%s'", prefixSnippet),
				Similarity: 100.0,
			})
			continue
		}

		foundKeys := rawHexKeyRegex.FindAllString(line, -1)
		for _, rawKey := range foundKeys {
			cleanKey := strings.ToLower(strings.TrimPrefix(rawKey, "0x"))

			if isZeroOrDummyKey(cleanKey) {
				continue
			}

			similarity := 100.0
			if refClean != "" {
				similarity = calculateSimilarity(cleanKey, refClean)
			}

			if similarity >= SimilarityThreshold {
				matches = append(matches, KeyMatch{
					FilePath:   filePath,
					LineNumber: lineNumber,
					LineText:   line,
					MatchedKey: rawKey,
					Similarity: similarity,
				})
			}
		}

		if len(foundKeys) == 0 && namedKeyRegex.MatchString(line) {
			matches = append(matches, KeyMatch{
				FilePath:   filePath,
				LineNumber: lineNumber,
				LineText:   line,
				MatchedKey: "Named PRIVATE_KEY pattern",
				Similarity: 100.0,
			})
		}
	}

	return matches, scanner.Err()
}

/******************************************************************************
 * Function Name : calculateSimilarity
 *
 * Purpose :
 *   Calculates percentage similarity between two strings using Levenshtein distance.
 *
 * Inputs :
 *   s1, s2
 *     Type        : string
 *     Description : Target strings to compare.
 *
 * Return :
 *   Type        : float64
 *   Description : Percentage match (0.0 - 100.0).
 *
 * Complexity :
 *   Time  : O(L1 * L2) where L1, L2 are string lengths.
 *   Space : O(L1)
 *
 * Error Cases :
 *   - Empty input strings return 0.0 similarity.
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func calculateSimilarity(s1, s2 string) float64 {
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}
	dist := levenshteinDistance(s1, s2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	return (1.0 - (float64(dist) / maxLen)) * 100.0
}

/******************************************************************************
 * Function Name : levenshteinDistance
 *
 * Purpose :
 *   Computes edit distance between two rune sequences.
 *
 * Inputs :
 *   s1, s2
 *     Type        : string
 *     Description : Target strings.
 *
 * Return :
 *   Type        : int
 *   Description : Total distance value.
 *
 * Complexity :
 *   Time  : O(N * M)
 *   Space : O(N)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   25
 ******************************************************************************/
func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	len1, len2 := len(r1), len(r2)

	column := make([]int, len1+1)
	for y := 1; y <= len1; y++ {
		column[y] = y
	}

	for x := 1; x <= len2; x++ {
		column[0] = x
		lastDiag := x - 1
		for y := 1; y <= len1; y++ {
			oldDiag := column[y]
			cost := 0
			if r1[y-1] != r2[x-1] {
				cost = 1
			}
			column[y] = min(column[y]+1, column[y-1]+1, lastDiag+cost)
			lastDiag = oldDiag
		}
	}
	return column[len1]
}

/******************************************************************************
 * Function Name : min
 *
 * Purpose :
 *   Returns the minimum of three integers.
 *
 * Inputs :
 *   a, b, c
 *     Type        : int
 *     Description : Values to evaluate.
 *
 * Return :
 *   Type        : int
 *   Description : Smallest value.
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
func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

/******************************************************************************
 * Function Name : isZeroOrDummyKey
 *
 * Purpose :
 *   Determines if a key consists of repeated or zero dummy characters.
 *
 * Inputs :
 *   key
 *     Type        : string
 *     Description : Hex string to check.
 *
 * Return :
 *   Type        : bool
 *   Description : True if key is dummy/placeholder.
 *
 * Complexity :
 *   Time  : O(N) where N is length of key.
 *   Space : O(1)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   15
 ******************************************************************************/
func isZeroOrDummyKey(key string) bool {
	if strings.Count(key, "0") > 50 {
		return true
	}
	first := key[0]
	for i := 1; i < len(key); i++ {
		if key[i] != first {
			return false
		}
	}
	return true
}

/******************************************************************************
 * Function Name : isBinaryExtension
 *
 * Purpose :
 *   Checks whether a filename matches common binary/archive extensions.
 *
 * Inputs :
 *   filename
 *     Type        : string
 *     Description : File name to evaluate.
 *
 * Return :
 *   Type        : bool
 *   Description : True if extension is binary.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None.
 *
 * Number Of Lines :
 *   12
 ******************************************************************************/
func isBinaryExtension(filename string) bool {
	ext := filepath.Ext(filename)
	switch ext {
	case ".rdb", ".exe", ".png", ".jpg", ".jpeg", ".zip", ".tar", ".gz", ".db", ".so", ".dylib", ".a", ".o":
		return true
	default:
		return false
	}
}