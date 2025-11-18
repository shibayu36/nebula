package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ListArgs はlistツールの引数を表す構造体
type ListArgs struct {
	Path      string `json:"path" description:"リストを取得するディレクトリのパス"`
	Recursive bool   `json:"recursive" description:"再帰的にディレクトリを探索するかどうか"`
}

// ListResult はlistツールの結果を表す構造体
type ListResult struct {
	Files []string `json:"files"`
	Error string   `json:"error,omitempty"`
}

// List は指定されたパス内のファイルとディレクトリをリストする
func List(args string) (string, error) {
	// argsにはどのツールでもJSONが入ってくるはずなので、JSONをパースしてListArgsに変換
	var listArgs ListArgs
	if err := json.Unmarshal([]byte(args), &listArgs); err != nil {
		return "", fmt.Errorf("引数の解析に失敗しました: %v", err)
	}

	var files []string

	if listArgs.Recursive {
		// 再帰的な探索
		err := filepath.Walk(listArgs.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err // エラーが発生した場合は中断
			}
			// 見つかったらパスをすべて配列に追加（ファイルもディレクトリも含む）
			files = append(files, path)
			return nil
		})
		if err != nil {
			// エラーが発生してもJSON形式で結果を返す
			result := ListResult{
				Files: []string{},
				Error: fmt.Sprintf("ディレクトリの探索に失敗しました: %v", err),
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), nil
		}
	} else {
		// 非再帰的な探索
		entries, err := os.ReadDir(listArgs.Path)
		if err != nil {
			result := ListResult{
				Files: []string{},
				Error: fmt.Sprintf("ディレクトリの読み込みに失敗しました: %v", err),
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), nil
		}

		// 各エントリのフルパスを構築して配列に追加
		for _, entry := range entries {
			files = append(files, filepath.Join(listArgs.Path, entry.Name()))
		}
	}

	// 成功時の結果をJSON形式で返す
	result := ListResult{
		Files: files,
		Error: "",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetListTool はlistツールの定義を返す
func GetListTool() ToolDefinition {
	return ToolDefinition{
		Schema: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list",
				Description: "指定したディレクトリ内のファイルとディレクトリの一覧を返します。recursiveがtrueの場合、再帰的にリストします。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "リストを取得するディレクトリのパス",
						},
						"recursive": {
							Type:        jsonschema.Boolean,
							Description: "再帰的にリストするかどうか（デフォルトはfalse）",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		Function: List,
	}
}
