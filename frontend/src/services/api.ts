// src/services/api.ts
import axios from "axios";

const apiClient = axios.create({
  baseURL: "http://localhost:8080",
});

export interface FileRecord {
  id: number;
  filename: string;
  size: number;
  mime_type: string;
  upload_date: string;
}

export const getFiles = () => apiClient.get<FileRecord[]>("/files");

export const uploadFile = (file: File) => {
  const formData = new FormData();
  formData.append("file", file);
  return apiClient.post("/upload", formData, {
    headers: { "Content-Type": "multipart/form-data" },
  });
};

export const deleteFile = (id: number) => apiClient.delete(`/files/${id}`);

// The download URL can be constructed directly, but you could also have a helper here
export const getDownloadUrl = (id: number) => `${apiClient.defaults.baseURL}/files/${id}/download`;