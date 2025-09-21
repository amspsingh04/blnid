// src/components/Uploader.tsx
import React from "react";
import { useDropzone } from "react-dropzone";
import { UploadCloud, LoaderCircle } from "lucide-react";

interface UploaderProps {
  onUpload: (files: File[]) => void;
  loading: boolean;
}

export function Uploader({ onUpload, loading }: UploaderProps) {
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: onUpload,
    disabled: loading,
  });

  return (
    <div
      {...getRootProps()}
      className={`p-10 border-2 border-dashed rounded-lg text-center cursor-pointer transition-colors
      ${isDragActive ? "border-blue-500 bg-blue-50" : "border-gray-300"}
      ${loading ? "cursor-not-allowed bg-gray-100" : "hover:bg-gray-50"}`}
    >
      <input {...getInputProps()} />
      {loading ? (
        <div className="flex flex-col items-center gap-2 text-gray-500">
          <LoaderCircle className="animate-spin" />
          <p>Uploading files...</p>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 text-gray-600">
          <UploadCloud size={32} />
          <p>
            {isDragActive
              ? "Drop the files here ..."
              : "Drag & drop files here, or click to select"}
          </p>
        </div>
      )}
    </div>
  );
}