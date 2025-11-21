package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/shibayu36/nebula/tools"
)

const maxToolCallSteps = 5

func main() {
	// 環境変数からAPIキーを取得
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: OPENAI_API_KEY environment variable is not set")
		fmt.Println("Please set your OpenAI API key: export OPENAI_API_KEY=your_api_key_here")
		os.Exit(1)
	}

	// OpenAIクライアントを初期化
	client := openai.NewClient(apiKey)

	// 利用可能なツールを取得
	tools := tools.GetAvailableTools()

	// ツールのスキーマを配列に変換
	var toolNames []string
	var toolSchemas []openai.Tool
	for name, tool := range tools {
		toolNames = append(toolNames, name)
		toolSchemas = append(toolSchemas, tool.Schema)
	}

	fmt.Println("nebula - OpenAI Chat CLI with Function Calling")
	fmt.Println("Available tools: " + strings.Join(toolNames, ", "))
	fmt.Println("Type 'exit' or 'quit' to end the conversation")
	fmt.Println("---")

	scanner := bufio.NewScanner(os.Stdin)

	// 会話履歴を保持
	messages := []openai.ChatCompletionMessage{
		// システムプロンプトを追加
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: getSystemPrompt(),
		},
	}

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		userInput := strings.TrimSpace(scanner.Text())

		// 終了コマンドをチェック
		if userInput == "exit" || userInput == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if userInput == "" {
			continue
		}

		// handleUserInputでユーザー入力1件を処理
		var err error
		messages, err = handleUserInput(client, userInput, messages, tools, toolSchemas)
		if err != nil {
			fmt.Printf("Error handling user input: %v\n", err)
			continue
		}
	}
}

// handleUserInput はユーザー入力1件を処理し、ツールコールがなくなるまで繰り返し実行する
func handleUserInput(
	client *openai.Client,
	userInput string,
	messages []openai.ChatCompletionMessage,
	tools map[string]tools.ToolDefinition,
	toolSchemas []openai.Tool,
) ([]openai.ChatCompletionMessage, error) {
	// ユーザーメッセージを履歴に追加
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	})

	// ツールコールがなくなるまでループ
	for step := 0; step < maxToolCallSteps; step++ {
		// OpenAI APIに送信
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:    openai.GPT5Nano,
				Messages: messages,
				Tools:    toolSchemas,
			},
		)
		if err != nil {
			return messages, fmt.Errorf("error calling OpenAI API: %v", err)
		}

		if len(resp.Choices) == 0 {
			return messages, fmt.Errorf("no response received from OpenAI")
		}

		responseMessage := resp.Choices[0].Message
		messages = append(messages, responseMessage)

		// ツールコールがない場合は最終応答として表示して終了
		if len(responseMessage.ToolCalls) == 0 {
			fmt.Printf("Assistant: %s\n\n", responseMessage.Content)
			return messages, nil
		}

		// ツールコールがある場合の処理
		fmt.Println("Assistant is using tools...")

		for _, toolCall := range responseMessage.ToolCalls {
			fmt.Printf("Tool call: %s, arguments: %s\n", toolCall.Function.Name, toolCall.Function.Arguments)

			if tool, exists := tools[toolCall.Function.Name]; exists {
				// ツール関数を実行
				result, err := tool.Function(toolCall.Function.Arguments)
				if err != nil {
					result = fmt.Sprintf(`{"error": "Tool execution failed: %v"}`, err)
				}

				// ツール実行結果をメッセージ履歴に追加
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})

				fmt.Printf("Tool '%s' executed with result: %s\n", toolCall.Function.Name, result)
			}
		}

		// ループを継続して、ツール実行結果を元に再度APIを呼び出す
	}

	return messages, fmt.Errorf("maximum tool call steps (%d) exceeded", maxToolCallSteps)
}
