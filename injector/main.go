package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// multiFlag allows collecting multiple flags
type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func detectFormat(filename string) string {
	if strings.HasSuffix(filename, ".json") {
		return "json"
	}
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		return "yaml"
	}
	return "yaml"
}

func jsonToYAML(jsonData []byte) ([]byte, error) {
	var m interface{}
	err := json.Unmarshal(jsonData, &m)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(m)
}

// processArrayPath processes array paths with special symbols for insertion
func processArrayPath(path string, modifiedJSON string) (string, error) {
	// Check for array insertion symbols: ^ for prepend, $ for append
	if strings.Contains(path, "^") {
		// Prepend to array: users^.name -> users.0.name
		path = strings.Replace(path, "^", ".0", 1)
	} else if strings.Contains(path, "$") {
		// Append to array: users$.name -> users.-1.name (sjson uses -1 for append)
		path = strings.Replace(path, "$", ".-1", 1)
	}
	return path, nil
}

// expandArrayRangePath expands array range paths like config.[*].sex or config.[0..1].sex
func expandArrayRangePath(path string, jsonData string) ([]string, error) {
	// Parse the path to get array path and field name
	arrayPath, fieldName, err := parseArrayRangePath(path)
	if err != nil {
		// No array range found, return original path
		return []string{path}, nil
	}

	// Find the array range pattern in the original path
	arrayRangeRegex := regexp.MustCompile(`\[(\*|\d+(?:\.\.\d+)?(?:,\d+(?:\.\.\d+)?)*)\]`)
	matches := arrayRangeRegex.FindStringSubmatch(path)
	if len(matches) == 0 {
		return []string{path}, nil
	}

	rangeExpr := matches[1]

	if rangeExpr == "*" {
		// Handle [*] - get all array indices
		return getAllArrayIndices(arrayPath, jsonData, fieldName)
	} else if strings.Contains(rangeExpr, ",") {
		// Handle [0,1,2] or [0..1,2] - specific indices or mixed
		return getSpecificIndices(arrayPath, rangeExpr, fieldName)
	} else if strings.Contains(rangeExpr, "..") {
		// Handle [0..1] - range expression
		return getRangeIndices(arrayPath, rangeExpr, fieldName)
	} else {
		// Single index like [0]
		index, err := strconv.Atoi(rangeExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid array index: %s", rangeExpr)
		}
		return []string{fmt.Sprintf("%s.%d.%s", arrayPath, index, fieldName)}, nil
	}
}

// parseArrayRangePath parses a path like "config.[*].sex" and returns the array path and field name
func parseArrayRangePath(path string) (arrayPath, fieldName string, err error) {
	// Find the array range pattern
	arrayRangeRegex := regexp.MustCompile(`\[(\*|\d+(?:\.\.\d+)?(?:,\d+(?:\.\.\d+)?)*)\]`)

	matches := arrayRangeRegex.FindStringSubmatchIndex(path)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("no array range found in path: %s", path)
	}

	// Extract the parts
	arrayPath = path[:matches[0]]
	fieldName = path[matches[1]:]

	// Clean up the paths - remove trailing dot from arrayPath and leading dot from fieldName
	arrayPath = strings.TrimSuffix(arrayPath, ".")
	fieldName = strings.TrimPrefix(fieldName, ".")

	return arrayPath, fieldName, nil
}

// getAllArrayIndices gets all indices for an array path
func getAllArrayIndices(basePath string, jsonData string, suffix string) ([]string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}

	// Navigate to the array
	pathParts := strings.Split(basePath, ".")
	current := data

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return nil, fmt.Errorf("path %s not found", basePath)
			}
		default:
			return nil, fmt.Errorf("path %s is not an object", basePath)
		}
	}

	// Check if current is an array
	array, ok := current.([]interface{})
	if !ok {
		return nil, fmt.Errorf("path %s is not an array", basePath)
	}

	// Generate paths for all indices
	var paths []string
	for i := 0; i < len(array); i++ {
		// For each array element, add the suffix field
		paths = append(paths, fmt.Sprintf("%s.%d.%s", basePath, i, suffix))
	}

	return paths, nil
}

// getRangeIndices gets indices for a range expression like "0..1"
func getRangeIndices(basePath, rangeExpr string, suffix string) ([]string, error) {
	parts := strings.Split(rangeExpr, "..")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range expression: %s", rangeExpr)
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid start index: %s", parts[0])
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid end index: %s", parts[1])
	}

	if start > end {
		return nil, fmt.Errorf("start index %d is greater than end index %d", start, end)
	}

	var paths []string
	for i := start; i <= end; i++ {
		paths = append(paths, fmt.Sprintf("%s.%d.%s", basePath, i, suffix))
	}

	return paths, nil
}

// getSpecificIndices gets indices for specific indices like "0,1,2"
func getSpecificIndices(basePath, indicesExpr string, suffix string) ([]string, error) {
	parts := strings.Split(indicesExpr, ",")
	var paths []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "..") {
			// Handle sub-ranges like "0..2" within "0,1..3,5"
			subPaths, err := getRangeIndices(basePath, part, suffix)
			if err != nil {
				return nil, err
			}
			paths = append(paths, subPaths...)
		} else {
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid index: %s", part)
			}
			paths = append(paths, fmt.Sprintf("%s.%d.%s", basePath, index, suffix))
		}
	}

	return paths, nil
}

func main() {
	filePath := flag.String("f", "", "input YAML/JSON file")
	outputFormat := flag.String("o", "yaml", "output format: yaml, json, or save (save to original file)")
	outputFile := flag.String("out", "", "output file (default: stdout)")
	var setExprs multiFlag
	flag.Var(&setExprs, "set", "set operation: path=value (can be used multiple times)")
	var insertExprs multiFlag
	flag.Var(&insertExprs, "insert", "insert operation: path=value (only insert if path doesn't exist)")
	var deleteExprs multiFlag
	flag.Var(&deleteExprs, "delete", "delete operation: path (can be used multiple times)")

	flag.Parse()

	// Show help if no file specified or no operations
	if *filePath == "" || (len(setExprs) == 0 && len(insertExprs) == 0 && len(deleteExprs) == 0) {
		fmt.Println("Usage: ./injector -f <file> [options]")
		fmt.Println("")
		fmt.Println("Operations:")
		fmt.Println("  --set path=value     Set value (replace if exists, create if not)")
		fmt.Println("  --insert path=value  Insert value (only if path doesn't exist)")
		fmt.Println("  --delete path        Delete value at path")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  ./injector -f data.yaml --set name=张三")
		fmt.Println("  ./injector -f data.yaml --set users.0.id=100 --set users.0.name=李四")
		fmt.Println("  ./injector -f data.yaml --set config.[*].sex=2")
		fmt.Println("  ./injector -f data.yaml --set config.[0..1].sex=2")
		fmt.Println("  ./injector -f data.yaml --set config.[0,2].sex=2")
		fmt.Println("  ./injector -f data.yaml --insert newField=value")
		fmt.Println("  ./injector -f data.yaml --delete oldField")
		fmt.Println("  ./injector -f data.yaml --set name=张三 -o json")
		fmt.Println("  ./injector -f data.yaml --set name=张三 -out output.yaml")
		fmt.Println("  ./injector -f data.yaml --set name=张三 -o save")
		os.Exit(1)
	}

	raw, err := ioutil.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	var jsonInput []byte
	inputFormat := detectFormat(*filePath)

	if inputFormat == "yaml" {
		var data interface{}
		err := yaml.Unmarshal(raw, &data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmarshalling YAML: %v\n", err)
			os.Exit(1)
		}
		intermediateJSON, err := json.Marshal(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling to intermediate JSON: %v\n", err)
			os.Exit(1)
		}
		jsonInput = intermediateJSON
	} else {
		jsonInput = raw
	}

	modifiedJSON := string(jsonInput)

	// Process set operations
	for _, expr := range setExprs {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: invalid set format '%s', expected path=value\n", expr)
			os.Exit(1)
		}

		path := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Process array range paths like config.[*].sex or config.[0..1].sex
		expandedPaths, err := expandArrayRangePath(path, modifiedJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding array range path %s: %v\n", path, err)
			os.Exit(1)
		}

		// Process each expanded path
		for _, expandedPath := range expandedPaths {
			// Process array insertion symbols
			processedPath, err := processArrayPath(expandedPath, modifiedJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing path %s: %v\n", expandedPath, err)
				os.Exit(1)
			}

			// Wrap value in quotes if it's not valid JSON
			if !json.Valid([]byte(value)) {
				value = fmt.Sprintf("%q", value)
			}

			modifiedJSON, err = sjson.SetRaw(modifiedJSON, processedPath, value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting %s: %v\n", processedPath, err)
				os.Exit(1)
			}
		}
	}

	// Process insert operations
	for _, expr := range insertExprs {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Error: invalid insert format '%s', expected path=value\n", expr)
			os.Exit(1)
		}

		path := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Process array insertion symbols
		path, err = processArrayPath(path, modifiedJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing path %s: %v\n", path, err)
			os.Exit(1)
		}

		// Simple check if path already exists
		if strings.Contains(modifiedJSON, fmt.Sprintf(`"%s"`, path)) {
			fmt.Fprintf(os.Stderr, "Warning: Path %s already exists, skipping insert\n", path)
			continue
		}

		// Wrap value in quotes if it's not valid JSON
		if !json.Valid([]byte(value)) {
			value = fmt.Sprintf("%q", value)
		}

		modifiedJSON, err = sjson.SetRaw(modifiedJSON, path, value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	// Process delete operations
	for _, expr := range deleteExprs {
		path := strings.TrimSpace(expr)

		// Process array insertion symbols
		path, err = processArrayPath(path, modifiedJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing path %s: %v\n", path, err)
			os.Exit(1)
		}

		modifiedJSON, err = sjson.Delete(modifiedJSON, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	// Prepare output
	var output []byte
	if *outputFormat == "json" {
		var m interface{}
		if err := json.Unmarshal([]byte(modifiedJSON), &m); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing modified JSON: %v\n", err)
			os.Exit(1)
		}
		prettyJSON, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling JSON: %v\n", err)
			os.Exit(1)
		}
		output = prettyJSON
	} else if *outputFormat == "save" {
		// Determine output format based on input file extension
		inputFormat := detectFormat(*filePath)
		if inputFormat == "json" {
			var m interface{}
			if err := json.Unmarshal([]byte(modifiedJSON), &m); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing modified JSON: %v\n", err)
				os.Exit(1)
			}
			prettyJSON, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshalling JSON: %v\n", err)
				os.Exit(1)
			}
			output = prettyJSON
		} else {
			yamlOut, err := jsonToYAML([]byte(modifiedJSON))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting to YAML: %v\n", err)
				os.Exit(1)
			}
			output = yamlOut
		}
	} else {
		yamlOut, err := jsonToYAML([]byte(modifiedJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting to YAML: %v\n", err)
			os.Exit(1)
		}
		output = yamlOut
	}

	// Output to file or stdout
	if *outputFormat == "save" {
		// Save to original file
		err := ioutil.WriteFile(*filePath, output, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to original file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully saved changes to %s\n", *filePath)
	} else if *outputFile != "" {
		err := ioutil.WriteFile(*outputFile, output, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully wrote output to %s\n", *outputFile)
	} else {
		fmt.Println(string(output))
	}
}
