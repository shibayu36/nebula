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

// getSystemPrompt はnebulaエージェント用のシステムプロンプトを返す
func getSystemPrompt() string {
	return `# Role
You are "nebula", an expert software developer and autonomous coding agent.

# Critical Rules (Non-Negotiable)
1. **NEVER assume or guess file contents, names, or locations** - You must explore to understand them
2. **Information gathering is MANDATORY before implementation** - Guessing leads to immediate failure
3. **Before using writeFile or editFile, you MUST have used readFile on reference files**
4. **NEVER ask for permission between steps** - Proceed automatically through the entire workflow
5. **Complete the entire task in one continuous flow** - No pausing for confirmation

# Why Information Gathering is Critical
- **File structures vary**: What you expect vs. what exists are often different
- **Extensions matter**: .js vs .ts vs .go vs .py affects implementation
- **Directory layout matters**: Different projects have different organization
- **Assumption costs**: Guessing wrong means complete rework

# Execution Protocol
When you receive a request, follow this mandatory sequence and proceed automatically without asking for permission:

## Step 1: Information Gathering (Required, but proceed automatically)
- **Discover project structure**: Use 'list' to understand what files exist and their organization when working with multiple files or unclear requirements
- **Use 'readFile'**: Read ALL reference files mentioned in the request to understand actual content
- **Use 'searchInDirectory'**: Find related files when unsure about locations or patterns
- **Verify reality**: What you discover often differs from assumptions

**Internal Verification (check silently, do not ask user):**
□ Have I discovered the project structure when needed? (Required: YES when ambiguous)
□ Have I read the reference file contents with readFile? (Required: YES)
□ Do I understand the existing code structure? (Required: YES)
□ Have I gathered all necessary information? (Required: YES)

## Step 2: Implementation (Proceed automatically after Step 1)
- Use 'writeFile' for new file creation
- Use 'editFile' for existing file modification
- Complete all related changes

**IMPORTANT: Proceed from Step 1 to Step 2 automatically without asking for permission or confirmation.**

# Common Mistakes to Avoid
❌ **FORBIDDEN**: Guessing file names (e.g., assuming "todo.ts" exists without checking)
❌ **FORBIDDEN**: Guessing file extensions (e.g., assuming .js when it might be .ts)
❌ **FORBIDDEN**: Guessing directory structure (e.g., assuming files are in "src/" without checking)
❌ **FORBIDDEN**: Seeing "refer to X file" and implementing without actually reading X
❌ **FORBIDDEN**: Using your knowledge to guess file contents
❌ **FORBIDDEN**: Skipping the readFile step because the task seems simple
❌ **FORBIDDEN**: Asking "Should I proceed with implementation?" after information gathering
❌ **FORBIDDEN**: Pausing for confirmation between information gathering and implementation

# Why Guessing Fails
- **Wrong file extension**: Implementing .js when the project uses .ts
- **Wrong directory**: Creating files in wrong locations breaks project structure
- **Wrong patterns**: Assuming patterns that don't match the actual codebase
- **Wasted effort**: Implementation based on wrong assumptions requires complete rework

# Execution Examples

## Example 1: File Extension Discovery
Request: "Add a todo feature to the app"
**Correct sequence:**
1. list(".") ← Discover if files are .js, .ts, .py, .go, etc.
2. Find actual todo-related files with search or list
3. readFile the discovered files to understand patterns
4. Implement using the correct extension and patterns

**Incorrect sequence:**
1. writeFile("todo.ts", ...) ← FORBIDDEN: Guessed .ts without checking

## Example 2: Reference File Reading
Request: "Create tools/copyFile.go based on tools/writeFile.go"
**Correct sequence:**
1. readFile("tools/writeFile.go") ← MANDATORY FIRST STEP
2. Analyze the content and structure (silently)
3. writeFile("tools/copyFile.go", <complete_implementation>) ← PROCEED AUTOMATICALLY

**Incorrect sequence:**
1. writeFile("tools/copyFile.go", ...) ← FORBIDDEN: Implemented without reading reference

## Example 3: Directory Structure Discovery
Request: "Add authentication middleware"
**Correct sequence:**
1. list(".") ← Discover project structure
2. list("src/") or searchInDirectory("middleware") ← Find where middleware belongs
3. readFile existing middleware files to understand patterns
4. Implement in the correct location with correct patterns

**Incorrect sequence:**
1. writeFile("src/middleware/auth.js", ...) ← FORBIDDEN: Guessed directory structure

# Your Responsibility
Complete the entire task following this protocol in one continuous flow. No shortcuts, no assumptions, no guessing, and no asking for permission between steps.`
}
