package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/shibayu36/nebula/memory"
	"github.com/shibayu36/nebula/tools"
)

const maxToolCallSteps = 5

func main() {
	// コマンドライン引数の解析
	listSessions := flag.Bool("list-sessions", false, "List recent sessions for current project")
	sessionID := flag.String("session", "", "Resume an existing session by ID")
	flag.Parse()

	// メモリ管理の初期化
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	dbPath := os.Getenv("NEBULA_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(homeDir, ".local", "share", "nebula", "memory.db")
	}

	manager, err := memory.NewManager(dbPath)
	if err != nil {
		fmt.Printf("Error: failed to initialize memory manager: %v\n", err)
		os.Exit(1)
	}
	defer manager.Close()

	// セッション一覧表示
	if *listSessions {
		sessions, err := manager.GetCurrentProjectSessions(20)
		if err != nil {
			fmt.Printf("Error: failed to get sessions: %v\n", err)
			os.Exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found for current project.")
			return
		}

		fmt.Println("Recent sessions:")
		fmt.Println("ID\t\t\tStarted At\t\t\tLast Message")
		fmt.Println(strings.Repeat("-", 100))
		for _, s := range sessions {
			lastMsg := s.LastMessage
			if len(lastMsg) > 50 {
				lastMsg = lastMsg[:50] + "..."
			}
			fmt.Printf("%s\t%s\t%s\n", s.ID, s.StartedAt.Format("2006-01-02 15:04:05"), lastMsg)
		}
		return
	}

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

	// セッションの開始または復元
	var messages []openai.ChatCompletionMessage

	if *sessionID != "" {
		// 既存セッションの復元
		session, err := manager.RestoreSession(*sessionID)
		if err != nil {
			fmt.Printf("Error: failed to restore session: %v\n", err)
			os.Exit(1)
		}

		// 過去のメッセージを取得
		memoryMessages, err := manager.GetSessionMessages(*sessionID)
		if err != nil {
			fmt.Printf("Error: failed to get session messages: %v\n", err)
			os.Exit(1)
		}

		// メッセージをOpenAI形式に変換
		messages = convertToOpenAIMessages(memoryMessages)
		// システムプロンプトを先頭に追加
		messages = append([]openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: getSystemPrompt(),
			},
		}, messages...)

		fmt.Printf("Resumed session: %s\n", session.ID)
	} else {
		// 新規セッションの開始
		projectPath, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error: failed to get current directory: %v\n", err)
			os.Exit(1)
		}

		session, err := manager.StartSession(projectPath, openai.GPT5Nano)
		if err != nil {
			fmt.Printf("Error: failed to start session: %v\n", err)
			os.Exit(1)
		}

		messages = []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: getSystemPrompt(),
			},
		}
		fmt.Printf("Started new session: %s\n", session.ID)
		fmt.Printf("Use --session %s to resume this session later\n", session.ID)
	}

	fmt.Println("nebula - OpenAI Chat CLI with Function Calling")
	fmt.Println("Available tools: " + strings.Join(toolNames, ", "))
	fmt.Println("Type 'exit' or 'quit' to end the conversation")
	fmt.Println("---")

	scanner := bufio.NewScanner(os.Stdin)

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
		messages, err = handleUserInput(client, userInput, messages, tools, toolSchemas, manager)
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
	manager *memory.Manager,
) ([]openai.ChatCompletionMessage, error) {
	// ユーザーメッセージを履歴に追加
	userMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	}
	messages = append(messages, userMsg)

	// ユーザーメッセージを永続化
	if err := manager.SaveMessage("user", userInput, nil, nil); err != nil {
		return messages, fmt.Errorf("failed to save user message: %w", err)
	}

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

		// アシスタントメッセージを永続化
		var toolCallsJSON string
		if len(responseMessage.ToolCalls) > 0 {
			toolCallsBytes, err := json.Marshal(responseMessage.ToolCalls)
			if err == nil {
				toolCallsJSON = string(toolCallsBytes)
			}
		}

		var toolCallsArg any
		if toolCallsJSON != "" {
			toolCallsArg = toolCallsJSON
		}

		if err := manager.SaveMessage("assistant", responseMessage.Content, toolCallsArg, nil); err != nil {
			return messages, fmt.Errorf("failed to save assistant message: %w", err)
		}

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
				toolMsg := openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				}
				messages = append(messages, toolMsg)

				// ツール実行結果を永続化
				if err := manager.SaveMessage("tool", result, nil, result); err != nil {
					return messages, fmt.Errorf("failed to save tool message: %w", err)
				}

				fmt.Printf("Tool '%s' executed with result: %s\n", toolCall.Function.Name, result)
			}
		}

		// ループを継続して、ツール実行結果を元に再度APIを呼び出す
	}

	return messages, fmt.Errorf("maximum tool call steps (%d) exceeded", maxToolCallSteps)
}

// convertToOpenAIMessages converts memory messages to OpenAI format
func convertToOpenAIMessages(memoryMessages []*memory.Message) []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage

	for _, msg := range memoryMessages {
		// Skip tool messages for now (they are complex to restore properly)
		if msg.Role == "tool" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages
}
