package tools

func GetAvailableTools() map[string]ToolDefinition {
	return map[string]ToolDefinition{
		"readFile":          GetReadFileTool(),
		"list":              GetListTool(),
		"searchInDirectory": GetSearchInDirectoryTool(),
		"writeFile":         GetWriteFileTool(),
		"editFile":          GetEditFileTool(),
	}
}
