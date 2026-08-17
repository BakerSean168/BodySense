import { useEffect, useState } from "react";
import { useParams } from "react-router";
import { consultationApi } from "../services/consultationService";
import type { Message, SharedConversation } from "../types/consultation";

/**
 * Public sharing is deliberately conversation-only. Diagnosis, BodyState and
 * Treatment are separate health-domain objects and are never silently exposed by
 * a chat share token.
 */
export function SharePage() {
  const { token } = useParams<{ token: string }>();
  const [data, setData] = useState<SharedConversation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    consultationApi
      .getSharedConversation(token)
      .then(setData)
      .catch((reason: unknown) =>
        setError(reason instanceof Error ? reason.message : "加载分享失败"),
      )
      .finally(() => setLoading(false));
  }, [token]);

  if (loading) return <div className="flex justify-center p-8">加载中...</div>;
  if (error)
    return <div className="flex justify-center p-8 text-red-500">{error}</div>;
  if (!data) return null;

  return (
    <div className="mx-auto max-w-2xl p-6">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-bold">BodySense 对话分享</h1>
        <p className="mt-1 text-gray-500">{data.title || "问诊记录"}</p>
        <p className="mt-2 text-xs text-gray-400">
          该链接只包含创建分享时的对话快照，不包含 BodyState、Diagnosis 或
          Treatment。
        </p>
      </div>

      <div className="space-y-4">
        {data.messages.map((message: Message) => (
          <div
            key={message.id}
            className={`flex ${message.role === "user" ? "justify-end" : "justify-start"}`}
          >
            <div
              className={`max-w-[80%] rounded-2xl px-4 py-2 ${
                message.role === "user"
                  ? "bg-primary-600 text-white"
                  : "bg-gray-100 text-gray-900"
              }`}
            >
              {message.parts
                .filter((part) => part.type === "text")
                .map((part, index) => (
                  <p key={index}>
                    {(part as { type: "text"; text: string }).text}
                  </p>
                ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
