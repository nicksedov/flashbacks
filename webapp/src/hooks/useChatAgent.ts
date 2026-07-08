import { useState, useCallback, useRef } from "react"
import type { Conversation, ChatMessage, SSEEvent } from "@/types"
import {
  createConversation,
  fetchConversations,
  deleteConversation,
  fetchConversationMessages,
  sendMessageStream,
} from "@/api/endpoints"

interface UseChatAgentReturn {
	conversation: Conversation | null
	conversations: Conversation[]
	messages: ChatMessage[]
	isStreaming: boolean
	error: string | null
	tokenCount: number
	maxTokens: number
	isTokenLimitReached: boolean
	currentImagePath: string | null
	// DeepSeek-specific extended usage info
	promptTokens: number
	completionTokens: number
	promptCacheHitTokens: number
	promptCacheMissTokens: number
	reasoningTokens: number
	createNewConversation: (imagePath?: string) => Promise<void>
	loadConversation: (id: number) => Promise<void>
	loadConversations: (imagePath?: string) => Promise<void>
	removeConversation: (id: number) => Promise<void>
	sendMessage: (content: string) => void
	abortStream: () => void
	resetForImage: (imagePath: string) => void
}

export function useChatAgent(language: string = "en"): UseChatAgentReturn {
  const [conversation, setConversation] = useState<Conversation | null>(null)
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentImagePath, setCurrentImagePath] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const tokenCount = conversation?.tokenCount ?? 0
  const maxTokens = conversation?.maxTokens ?? 0
  const isTokenLimitReached = maxTokens > 0 && tokenCount >= maxTokens
 
  // DeepSeek-specific extended usage state
  const [promptTokens, setPromptTokens] = useState(0)
  const [completionTokens, setCompletionTokens] = useState(0)
  const [promptCacheHitTokens, setPromptCacheHitTokens] = useState(0)
  const [promptCacheMissTokens, setPromptCacheMissTokens] = useState(0)
  const [reasoningTokens, setReasoningTokens] = useState(0)

  const createNewConversation = useCallback(async (imagePath?: string) => {
    try {
      const conv = await createConversation({ imagePath, language })
      setConversation(conv)
      setMessages([])
      setError(null)
      setCurrentImagePath(imagePath ?? null)
      // Refresh conversations list
      const list = await fetchConversations(imagePath)
      setConversations(list)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create conversation")
    }
  }, [language])

  const loadConversations = useCallback(async (imagePath?: string) => {
    try {
      const list = await fetchConversations(imagePath)
      setConversations(list)
      setCurrentImagePath(imagePath ?? null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load conversations")
    }
  }, [])

  const loadConversation = useCallback(async (id: number) => {
    try {
      const list = await fetchConversations(currentImagePath ?? undefined)
      const conv = list.find(c => c.id === id)
      if (conv) {
        setConversation(conv)
        const msgs = await fetchConversationMessages(id)
        setMessages(msgs)
        setError(null)
        setConversations(list)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load conversation")
    }
  }, [currentImagePath])

  const removeConversation = useCallback(async (id: number) => {
    try {
      await deleteConversation(id)
      if (conversation?.id === id) {
        setConversation(null)
        setMessages([])
      }
      // Refresh conversations list
      const list = await fetchConversations(currentImagePath ?? undefined)
      setConversations(list)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete conversation")
    }
  }, [conversation, currentImagePath])

  const abortStream = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
      setIsStreaming(false)
    }
  }, [])

  const resetForImage = useCallback((imagePath: string) => {
    setConversation(null)
    setMessages([])
    setConversations([])
    setError(null)
    setCurrentImagePath(imagePath)
  }, [])

  const sendMessage = useCallback((content: string) => {
    if (!conversation || isStreaming) return

    // Block when token limit reached
    if (isTokenLimitReached) {
      setError("Token limit reached. Start a new conversation to continue.")
      return
    }

    setError(null)
    setIsStreaming(true)

    // Add user message optimistically
    const userMsg: ChatMessage = {
      id: Date.now(), // temporary ID
      role: "user",
      content,
      createdAt: new Date().toISOString(),
    }
    setMessages(prev => [...prev, userMsg])

    // Track streaming assistant message
    let assistantContent = ""
    const toolCallStates: Array<{ name: string; status: string; result: string }> = []

    const abortController = new AbortController()
    abortRef.current = abortController

    sendMessageStream(
      conversation.id,
      content,
      (event: SSEEvent) => {
        switch (event.type) {
          case "tool_call":
            toolCallStates.push({ name: event.name, status: event.status, result: "" })
            // Update messages with tool call indicator
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant" && last.id < 0) {
                // Update existing streaming assistant message
                updated[updated.length - 1] = {
                  ...last,
                  toolCalls: toolCallStates.map(tc => ({
                    name: tc.name,
                    arguments: "",
                    result: tc.result,
                  })),
                }
              } else {
                // Create new streaming assistant message with tool call
                updated.push({
                  id: -1,
                  role: "assistant",
                  content: "",
                  toolCalls: toolCallStates.map(tc => ({
                    name: tc.name,
                    arguments: "",
                    result: tc.result,
                  })),
                  createdAt: new Date().toISOString(),
                })
              }
              return updated
            })
            break

          case "tool_result": {
            // Update tool call result
            const tc = toolCallStates.find(t => t.name === event.name && t.status !== "completed")
            if (tc) {
              tc.status = "completed"
              tc.result = event.result
            }
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant") {
                updated[updated.length - 1] = {
                  ...last,
                  toolCalls: toolCallStates.map(t => ({
                    name: t.name,
                    arguments: "",
                    result: t.result,
                  })),
                }
              }
              return updated
            })
            break
          }

          case "message":
            assistantContent = event.content
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant" && last.id < 0) {
                updated[updated.length - 1] = {
                  ...last,
                  content: assistantContent,
                }
              } else {
                updated.push({
                  id: -1,
                  role: "assistant",
                  content: assistantContent,
                  toolCalls: toolCallStates.length > 0
                    ? toolCallStates.map(tc => ({ name: tc.name, arguments: "", result: tc.result }))
                    : undefined,
                  createdAt: new Date().toISOString(),
                })
              }
              return updated
            })
            break

          case "token_usage":
            // Update conversation token state
            setConversation(prev => prev ? {
              ...prev,
              tokenCount: event.tokenCount,
              maxTokens: event.maxTokens,
            } : prev)
            // Update DeepSeek-specific extended usage
            if (event.promptTokens !== undefined) setPromptTokens(event.promptTokens)
            if (event.completionTokens !== undefined) setCompletionTokens(event.completionTokens)
            if (event.promptCacheHitTokens !== undefined) setPromptCacheHitTokens(event.promptCacheHitTokens)
            if (event.promptCacheMissTokens !== undefined) setPromptCacheMissTokens(event.promptCacheMissTokens)
            if (event.reasoningTokens !== undefined) setReasoningTokens(event.reasoningTokens)
            break

          case "error":
            setError(event.error)
            setIsStreaming(false)
            break

          case "done":
            setIsStreaming(false)
            // Note: We intentionally do NOT reload messages from the server here.
            // The server stores tool call info (name/args) and tool results (role "tool")
            // as separate messages, losing the merged toolCalls[].result data that was
            // built during streaming. Reloading would make thumbnails disappear.
            // Refresh conversations list to sync server-side updates (e.g. title generated from first user message)
            fetchConversations(conversation?.imagePath ?? undefined)
              .then(list => {
                setConversations(list)
                const updated = list.find(c => c.id === conversation?.id)
                if (updated) setConversation(updated)
              })
              .catch(() => {})
            break
        }
      },
      abortController.signal,
    )
  }, [conversation, isStreaming, isTokenLimitReached])

  return {
  	conversation,
  	conversations,
  	messages,
  	isStreaming,
  	error,
  	tokenCount,
  	maxTokens,
  	isTokenLimitReached,
  	currentImagePath,
  	promptTokens,
  	completionTokens,
  	promptCacheHitTokens,
  	promptCacheMissTokens,
  	reasoningTokens,
  	createNewConversation,
  	loadConversation,
  	loadConversations,
  	removeConversation,
  	sendMessage,
  	abortStream,
  	resetForImage,
  }
}
