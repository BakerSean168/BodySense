import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useUploadStore } from "@/stores/uploadStore";
import type { UserUpload } from "../../../types/upload.types";
import { UploadStep } from "./UploadStep";

const report: UserUpload = {
  id: "upload-1",
  file_type: "report",
  original_name: "体检报告.pdf",
  file_size: 1024,
  mime_type: "application/pdf",
  source_availability: "available",
  ocr_result: null,
  ocr_status: "completed",
  created_at: "2026-11-01T00:00:00Z",
  updated_at: "2026-11-01T00:00:00Z",
};

describe("UploadStep original removal", () => {
  const fetchUploads = vi.fn();
  const deleteUpload = vi.fn();

  beforeEach(() => {
    fetchUploads.mockReset().mockResolvedValue(undefined);
    deleteUpload.mockReset().mockResolvedValue(undefined);
    useUploadStore.setState({
      uploads: [report],
      fetchUploads,
      deleteUpload,
    });
  });

  it("asks in an in-app dialog before the original is removed", async () => {
    render(<UploadStep />);

    expect(screen.queryByText("移除这个文件？")).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: `删除 ${report.original_name}` }),
    );

    expect(screen.getByText("移除这个文件？")).toBeInTheDocument();
    expect(
      screen.getByText(/原始文件会被永久移除，不能再查看或核验/),
    ).toBeInTheDocument();
    expect(deleteUpload).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "移除原件" }));

    await waitFor(() => {
      expect(deleteUpload).toHaveBeenCalledWith(report.id);
    });
    await waitFor(() => {
      expect(screen.queryByText("移除这个文件？")).not.toBeInTheDocument();
    });
  });

  it("leaves the file alone when the confirmation is cancelled", async () => {
    render(<UploadStep />);

    fireEvent.click(
      screen.getByRole("button", { name: `删除 ${report.original_name}` }),
    );
    fireEvent.click(screen.getByRole("button", { name: "取消" }));

    await waitFor(() => {
      expect(screen.queryByText("移除这个文件？")).not.toBeInTheDocument();
    });
    expect(deleteUpload).not.toHaveBeenCalled();
  });

  it("keeps the dialog open and reports a failed removal", async () => {
    deleteUpload.mockRejectedValueOnce(new Error("移除失败，请重试。"));
    render(<UploadStep />);

    fireEvent.click(
      screen.getByRole("button", { name: `删除 ${report.original_name}` }),
    );
    fireEvent.click(screen.getByRole("button", { name: "移除原件" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "移除失败，请重试。",
    );
    expect(screen.getByText("移除这个文件？")).toBeInTheDocument();
  });

  it("shows a removed original without offering removal again", () => {
    useUploadStore.setState({
      uploads: [{ ...report, source_availability: "source_unavailable" }],
    });
    render(<UploadStep />);

    expect(screen.getByText("原件已移除")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: `删除 ${report.original_name}` }),
    ).not.toBeInTheDocument();
  });
});
