// src/components/FileListItem.tsx
import React from "react";
import { FileRecord, getDownloadUrl } from "../services/api";
import { formatBytes } from "../utils/formatBytes";
import { Download, Trash2 } from "lucide-react";

interface FileListItemProps {
  file: FileRecord;
  onDelete: (id: number) => void;
}

export function FileListItem({ file, onDelete }: FileListItemProps) {
  return (
    <li className="p-4 flex justify-between items-center hover:bg-gray-50 transition-colors">
      <div>
        <p className="font-medium text-gray-800">{file.filename}</p>
        <p className="text-sm text-gray-500">
          {formatBytes(file.size)} • {file.mime_type} • Uploaded:{" "}
          {new Date(file.upload_date).toLocaleString()}
        </p>
      </div>
      <div className="flex gap-2">
        <a
          href={getDownloadUrl(file.id)}
          className="p-2 text-blue-600 rounded-md hover:bg-blue-100 hover:text-blue-700 transition-colors"
          title="Download"
        >
          <Download size={20} />
        </a>
        <button
          onClick={() => onDelete(file.id)}
          className="p-2 text-red-600 rounded-md hover:bg-red-100 hover:text-red-700 transition-colors"
          title="Delete"
        >
          <Trash2 size={20} />
        </button>
      </div>
    </li>
  );
}