use crate::{ClaudeProcess, ClaudeSetup};
use axum::{
    extract::State,
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use tower_http::cors::CorsLayer;
use tracing::{info, warn};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionRequest {
    pub model: String,
    pub messages: Vec<ChatMessage>,
    #[serde(default)]
    pub tools: Option<Vec<Tool>>,
    #[serde(default)]
    pub tool_choice: Option<ToolChoice>,
    #[serde(default)]
    pub temperature: Option<f32>,
    #[serde(default)]
    pub max_tokens: Option<u32>,
    #[serde(default)]
    pub stream: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: String,
    pub content: Option<String>,
    #[serde(default)]
    pub tool_calls: Option<Vec<ToolCall>>,
    #[serde(default)]
    pub tool_call_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tool {
    #[serde(rename = "type")]
    pub tool_type: String,
    pub function: FunctionDefinition,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionDefinition {
    pub name: String,
    pub description: Option<String>,
    pub parameters: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ToolChoice {
    Auto,
    None,
    Required,
    Function { function: FunctionSpec },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionSpec {
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    #[serde(rename = "type")]
    pub tool_type: String,
    pub function: FunctionCall,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionCall {
    pub name: String,
    pub arguments: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionResponse {
    pub id: String,
    pub object: String,
    pub created: u64,
    pub model: String,
    pub choices: Vec<Choice>,
    pub usage: Usage,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Choice {
    pub index: i32,
    pub message: ChatMessage,
    pub finish_reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Usage {
    pub prompt_tokens: u32,
    pub completion_tokens: u32,
    pub total_tokens: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelsResponse {
    pub object: String,
    pub data: Vec<Model>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Model {
    pub id: String,
    pub object: String,
    pub created: u64,
    pub owned_by: String,
}

pub struct AppState {
    claude_setup: Arc<ClaudeSetup>,
    processes: RwLock<HashMap<String, ClaudeProcess>>,
}

impl AppState {
    pub fn new(claude_setup: Arc<ClaudeSetup>) -> Self {
        Self {
            claude_setup,
            processes: RwLock::new(HashMap::new()),
        }
    }
}

pub async fn start_server(claude_setup: Arc<ClaudeSetup>, port: u16) -> crate::Result<()> {
    let app_state = Arc::new(AppState::new(claude_setup));

    let app = Router::new()
        .route("/v1/chat/completions", post(chat_completions))
        .route("/v1/models", get(list_models))
        .route("/health", get(health_check))
        .layer(CorsLayer::permissive())
        .with_state(app_state);

    let addr = format!("0.0.0.0:{}", port);
    info!("🚀 Claude Relay OpenAI-compatible server starting on {}", addr);
    info!("📡 API endpoints:");
    info!("   POST http://localhost:{}/v1/chat/completions", port);
    info!("   GET  http://localhost:{}/v1/models", port);
    info!("   GET  http://localhost:{}/health", port);

    let listener = tokio::net::TcpListener::bind(&addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}

async fn health_check() -> Json<serde_json::Value> {
    Json(serde_json::json!({
        "status": "ok",
        "service": "clay",
        "version": "0.1.0"
    }))
}

async fn list_models() -> Json<ModelsResponse> {
    Json(ModelsResponse {
        object: "list".to_string(),
        data: vec![
            Model {
                id: "claude-3-5-sonnet-20241022".to_string(),
                object: "model".to_string(),
                created: 1640995200,
                owned_by: "anthropic".to_string(),
            },
            Model {
                id: "claude-3-5-haiku-20241022".to_string(),
                object: "model".to_string(),
                created: 1640995200,
                owned_by: "anthropic".to_string(),
            },
            Model {
                id: "claude-3-opus-20240229".to_string(),
                object: "model".to_string(),
                created: 1640995200,
                owned_by: "anthropic".to_string(),
            },
        ],
    })
}

async fn chat_completions(
    State(state): State<Arc<AppState>>,
    Json(request): Json<ChatCompletionRequest>,
) -> std::result::Result<Json<ChatCompletionResponse>, StatusCode> {
    // Get or create a Claude process
    let process_id = "default"; // For now, use a single process
    let mut processes = state.processes.write().await;
    
    if !processes.contains_key(process_id) {
        match ClaudeProcess::new(state.claude_setup.clone()) {
            Ok(process) => {
                processes.insert(process_id.to_string(), process);
            }
            Err(e) => {
                warn!("Failed to create Claude process: {}", e);
                return Err(StatusCode::INTERNAL_SERVER_ERROR);
            }
        }
    }

    let process = processes.get_mut(process_id).unwrap();

    // Convert OpenAI messages to Claude prompt
    let prompt = build_claude_prompt(&request.messages, &request.tools);
    
    // Send message to Claude
    let response_text = match process.send_message(&prompt) {
        Ok(text) => text,
        Err(e) => {
            warn!("Failed to send message to Claude: {}", e);
            return Err(StatusCode::INTERNAL_SERVER_ERROR);
        }
    };

    // Parse response for tool calls if needed
    let (content, tool_calls) = parse_claude_response(&response_text, &request.tools);

    // Build OpenAI-compatible response
    let response = ChatCompletionResponse {
        id: format!("chatcmpl-{}", Uuid::new_v4()),
        object: "chat.completion".to_string(),
        created: chrono::Utc::now().timestamp() as u64,
        model: request.model,
        choices: vec![Choice {
            index: 0,
            message: ChatMessage {
                role: "assistant".to_string(),
                content: if content.is_empty() { None } else { Some(content) },
                tool_calls,
                tool_call_id: None,
            },
            finish_reason: "stop".to_string(),
        }],
        usage: Usage {
            prompt_tokens: estimate_tokens(&prompt),
            completion_tokens: estimate_tokens(&response_text),
            total_tokens: estimate_tokens(&prompt) + estimate_tokens(&response_text),
        },
    };

    Ok(Json(response))
}

fn build_claude_prompt(messages: &[ChatMessage], tools: &Option<Vec<Tool>>) -> String {
    let mut prompt = String::new();

    // Add tool definitions if provided
    if let Some(tools) = tools {
        if !tools.is_empty() {
            prompt.push_str("You have access to the following tools:\n\n");
            for tool in tools {
                prompt.push_str(&format!("## {}\n", tool.function.name));
                if let Some(desc) = &tool.function.description {
                    prompt.push_str(&format!("Description: {}\n", desc));
                }
                if let Some(params) = &tool.function.parameters {
                    prompt.push_str(&format!("Parameters: {}\n", serde_json::to_string_pretty(params).unwrap_or_default()));
                }
                prompt.push('\n');
            }
            prompt.push_str("When you need to use a tool, respond with a JSON object in this format:\n");
            prompt.push_str("```json\n{\"tool_calls\": [{\"function\": {\"name\": \"function_name\", \"arguments\": {\"param\": \"value\"}}}]}\n```\n\n");
        }
    }

    // Add conversation messages
    for message in messages {
        match message.role.as_str() {
            "system" => {
                if let Some(content) = &message.content {
                    prompt.push_str(&format!("System: {}\n\n", content));
                }
            }
            "user" => {
                if let Some(content) = &message.content {
                    prompt.push_str(&format!("User: {}\n\n", content));
                }
            }
            "assistant" => {
                if let Some(content) = &message.content {
                    prompt.push_str(&format!("Assistant: {}\n\n", content));
                }
                if let Some(tool_calls) = &message.tool_calls {
                    for tool_call in tool_calls {
                        prompt.push_str(&format!("Tool Call: {} with arguments: {}\n\n", 
                            tool_call.function.name, tool_call.function.arguments));
                    }
                }
            }
            "tool" => {
                if let Some(content) = &message.content {
                    prompt.push_str(&format!("Tool Result: {}\n\n", content));
                }
            }
            _ => {}
        }
    }

    prompt
}

fn parse_claude_response(response: &str, tools: &Option<Vec<Tool>>) -> (String, Option<Vec<ToolCall>>) {
    // Check if response contains tool calls
    if let Some(_) = tools {
        if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(response) {
            if let Some(tool_calls_value) = parsed.get("tool_calls") {
                if let Ok(tool_calls) = serde_json::from_value::<Vec<serde_json::Value>>(tool_calls_value.clone()) {
                    let mut converted_calls = Vec::new();
                    
                    for call in tool_calls {
                        if let (Some(function), Some(name)) = (call.get("function"), call.get("function").and_then(|f| f.get("name"))) {
                            let tool_call = ToolCall {
                                id: format!("call_{}", Uuid::new_v4()),
                                tool_type: "function".to_string(),
                                function: FunctionCall {
                                    name: name.as_str().unwrap_or("").to_string(),
                                    arguments: function.get("arguments")
                                        .map(|args| serde_json::to_string(args).unwrap_or_default())
                                        .unwrap_or_default(),
                                },
                            };
                            converted_calls.push(tool_call);
                        }
                    }
                    
                    if !converted_calls.is_empty() {
                        return (String::new(), Some(converted_calls));
                    }
                }
            }
        }
    }

    // No tool calls found, return content as-is
    (response.to_string(), None)
}

fn estimate_tokens(text: &str) -> u32 {
    // Rough estimation: ~4 characters per token
    (text.len() / 4).max(1) as u32
}