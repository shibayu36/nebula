package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// SearchInDirectoryArgs はsearchInDirectoryツールの引数を表す構造体
type SearchInDirectoryArgs struct {
	Path         string   `json:"path" description:"検索するディレクトリのパス"`
	Keyword      string   `json:"keyword" description:"検索するキーワード"`
	ExcludePaths []string `json:"excludePaths,omitempty" description:"除外するパスのパターン（先頭一致）"`
}

// SearchInDirectoryResult はsearchInDirectoryツールの結果を表す構造体
type SearchInDirectoryResult struct {
	Files []string `json:"files"`
	Error string   `json:"error,omitempty"`
}

// SearchInDirectory は指定されたディレクトリ配下を再帰的に検索し、キーワードを含むファイルを見つける
func SearchInDirectory(args string) (string, error) {
	// argsにはどのツールでもJSONが入ってくるはずなので、JSONをパースしてSearchInDirectoryArgsに変換
	var searchInDirectoryArgs SearchInDirectoryArgs
	if err := json.Unmarshal([]byte(args), &searchInDirectoryArgs); err != nil {
		return "", fmt.Errorf("引数の解析に失敗しました: %v", err)
	}

	var files []string

	// ディレクトリ以下のすべてのファイルを走査
	err := filepath.Walk(searchInDirectoryArgs.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // エラーが発生した場合は中断
		}

		// excludePathsによる除外チェック
		if len(searchInDirectoryArgs.ExcludePaths) > 0 {
			for _, excludePath := range searchInDirectoryArgs.ExcludePaths {
				if strings.HasPrefix(path, excludePath) {
					// ディレクトリの場合は配下をスキップ
					if info.IsDir() {
						return filepath.SkipDir
					}
					// ファイルの場合はスキップ
					return nil
				}
			}
		}

		// ディレクトリは検索対象外
		if info.IsDir() {
			return nil
		}

		// ファイルを開いて読み込み
		file, err := os.Open(path)
		if err != nil {
			// バイナリファイルや権限なしファイルは静かにスキップ
			// エラーを返すと全体の検索が止まってしまう
			return nil
		}
		defer file.Close()

		// ファイルの内容を読み込んでキーワードを検索
		// bufio.Scannerを使って効率的に読み込み
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), searchInDirectoryArgs.Keyword) {
				files = append(files, path)
				break // 1つのファイルで複数行マッチしても1回だけ記録
			}
		}

		return nil
	})

	// 検索処理でエラーが発生した場合はJSON形式で結果を返す
	if err != nil {
		result := SearchInDirectoryResult{
			Files: []string{},
			Error: fmt.Sprintf("検索処理中にエラーが発生しました: %v", err),
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil
	}

	// 成功時の結果をJSON形式で返す
	result := SearchInDirectoryResult{
		Files: files,
		Error: "",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetSearchInDirectoryTool はsearchInDirectoryツールの定義を返す
func GetSearchInDirectoryTool() ToolDefinition {
	return ToolDefinition{
		Schema: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "searchInDirectory",
				Description: "指定したディレクトリ内を再帰的に検索し、キーワードを含むファイルを見つけます。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "検索するディレクトリのパス",
						},
						"keyword": {
							Type:        jsonschema.String,
							Description: "検索するキーワード",
						},
						"excludePaths": {
							Type:        jsonschema.Array,
							Description: "除外するパスのパターン（先頭一致）。指定されたパターンで始まるパスは検索対象から除外されます。",
							Items: &jsonschema.Definition{
								Type: jsonschema.String,
							},
						},
					},
					Required: []string{"path", "keyword"},
				},
			},
		},
		Function: SearchInDirectory,
	}
}
