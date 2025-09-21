// src/components/FileList.tsx
import React from "react";
import { FileRecord } from "../services/api";
import { FileListItem } from "./fileListItem";

interface FileListProps {
  files: FileRecord[];
  onDelete: (id: number) => void;
}

export function FileList({ files, onDelete }: FileListProps) {
  if (files.length === 0) {
    return (
      <p className="text-gray-500 text-center py-10">No files found.</p>
    );
  }
  return (
    <ul className="divide-y border rounded-lg">{files.map((file) => (
        <FileListItem key={file.id} file={file} onDelete={onDelete} />
    ))}</ul>
  );
}