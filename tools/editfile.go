package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// EditFileArgs はeditFileツールの引数を表す構造体
type EditFileArgs struct {
	Path       string `json:"path" description:"編集するファイルのパス"`
	NewContent string `json:"new_content" description:"ファイルの新しい内容（完全な内容）"`
}

// EditFileResult はeditFileツールの結果を表す構造体
type EditFileResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// EditFile は既存ファイルの内容を完全に上書きする（ユーザー許可が必要）
func EditFile(args string) (string, error) {
	// argsにはどのツールでもJSONが入ってくるはずなので、JSONをパースしてEditFileArgsに変換
	var editFileArgs EditFileArgs
	if err := json.Unmarshal([]byte(args), &editFileArgs); err != nil {
		return "", fmt.Errorf("引数の解析に失敗しました: %v", err)
	}

	genErrorResult := func(errorMessage string) string {
		result := EditFileResult{
			Success: false,
			Error:   errorMessage,
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON)
	}

	// ファイルが存在するかチェック
	if _, err := os.Stat(editFileArgs.Path); err != nil {
		return genErrorResult(fmt.Sprintf("ファイルが存在しません。新しいファイルの作成にはwriteFileを使用してください。: %v", err)), nil
	}

	// ユーザー許可の取得
	fmt.Printf("\nファイルを編集します: %s\n", editFileArgs.Path)
	fmt.Printf("--- 内容 ---\n%s\n\n", editFileArgs.NewContent)
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

	// ファイルに内容を書き込む
	file, err := os.Create(editFileArgs.Path)
	if err != nil {
		return genErrorResult(fmt.Sprintf("ファイルのオープンに失敗しました: %v", err)), nil
	}
	defer file.Close()

	if _, err := file.WriteString(editFileArgs.NewContent); err != nil {
		return genErrorResult(fmt.Sprintf("ファイルへの書き込みに失敗しました: %v", err)), nil
	}

	result := EditFileResult{
		Success: true,
		Error:   "",
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetEditFileTool はeditFileツールの定義を返す
func GetEditFileTool() ToolDefinition {
	return ToolDefinition{
		Schema: openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "editFile",
				Description: "既存ファイルの内容を完全に上書きします。重要: ファイルを破壊しないために、必ず以下のワークフローに従ってください: 1. 'readFile'を使用して現在の完全な内容を取得する。2. 思考プロセスで、読み取った内容を基に新しいファイルの完全版を構築する。3. このツールを使用して完全な新しい内容を書き込む。部分的な編集には使用しないでください。常にファイル全体の内容を提供してください。",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path": {
							Type:        jsonschema.String,
							Description: "編集する既存ファイルのパス",
						},
						"new_content": {
							Type:        jsonschema.String,
							Description: "既存ファイル全体を上書きする新しい完全な内容",
						},
					},
					Required: []string{"path", "new_content"},
				},
			},
		},
		Function: EditFile,
	}
}
