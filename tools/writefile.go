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

// WriteFileArgs はwriteFileツールの引数を表す構造体
type WriteFileArgs struct {
	Path    string `json:"path" description:"書き込むファイルのパス"`
	Content string `json:"content" description:"書き込む内容"`
}

// WriteFileResult はwriteFileツールの結果を表す構造体
type WriteFileResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// WriteFile は指定されたパスに新しいファイルを作成する（ユーザー許可が必要）
func WriteFile(args string) (string, error) {
	// argsにはどのツールでもJSONが入ってくるはずなので、JSONをパースしてWriteFileArgsに変換
	var writeFileArgs WriteFileArgs
	if err := json.Unmarshal([]byte(args), &writeFileArgs); err != nil {
		return "", fmt.Errorf("引数の解析に失敗しました: %v", err)
	}

	genErrorResult := func(errorMessage string) string {
		result := WriteFileResult{
			Success: false,
			Error:   errorMessage,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON)
	}

	// 安全性チェック: 既存ファイルの上書きを防止
	if _, err := os.Stat(writeFileArgs.Path); err == nil {
		return genErrorResult(fmt.Sprintf("ファイルが既に存在します。既存ファイルの編集にはeditFileを使用してください: %s", writeFileArgs.Path)), nil
	}

	// ユーザー許可の取得
	fmt.Printf("\n新しいファイルを作成します: %s\n", writeFileArgs.Path)
	fmt.Printf("--- 内容 ---\n%s\n\n", writeFileArgs.Content)
	fmt.Print("実行してもよろしいですか？(y/N): ")

	// ユーザー応答を読み取り
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return genErrorResult("ユーザー応答の読み取りに失敗しました"), nil
	}
	// yまたはY以外はキャンセル扱い
	response := strings.TrimSpace(scanner.Text())
	if response != "y" && response != "Y" {
		return genErrorResult("ユーザーによってキャンセルされました"), nil
	}

	// 親ディレクトリの自動作成
	if err := os.MkdirAll(filepath.Dir(writeFileArgs.Path), 0755); err != nil {
		return genErrorResult(fmt.Sprintf("親ディレクトリの作成に失敗しました: %v", err)), nil
	}

	// ファイルを作成
	file, err := os.Create(writeFileArgs.Path)
	if err != nil {
		return genErrorResult(fmt.Sprintf("ファイルの作成に失敗しました: %v", err)), nil
	}
	defer file.Close()

	// ファイルに内容を書き込む
	if _, err := file.WriteString(writeFileArgs.Content); err != nil {
		return genErrorResult(fmt.Sprintf("ファイルへの書き込みに失敗しました: %v", err)), nil
	}

	// 成功時の結果を返却
	result := WriteFileResult{
		Success: true,
		Error:   "",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetWriteFileTool はwriteFileツールの定義を返す
func GetWriteFileTool() ToolDefinition {
	return ToolDefinition{
		Schema: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "writeFile",
				Description: "指定されたパスに新しいファイルを作成し、内容を書き込みます",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "作成するファイルの完全なパス",
						},
						"content": {
							Type:        jsonschema.String,
							Description: "ファイルに書き込む内容",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		Function: WriteFile,
	}
}
