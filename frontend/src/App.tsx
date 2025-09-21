// src/App.tsx
import React, { useEffect, useState, useMemo } from "react";
import { FileRecord, getFiles, uploadFile, deleteFile } from "./services/api";
import { Toaster, toast } from "react-hot-toast";
import { Uploader } from "./components/uploader";
import { FileList } from "./components/fileList";
import { SearchBar } from "./components/searchBar";

function App() {
  const [files, setFiles] = useState<FileRecord[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(false);

  const fetchFiles = async () => {
    try {
      const res = await getFiles();
      setFiles(res.data || []);
    } catch (err) {
      toast.error("Failed to fetch files.");
      setFiles([]);
    }
  };

  useEffect(() => {
    fetchFiles();
  }, []);

  const handleUpload = async (acceptedFiles: File[]) => {
    setLoading(true);
    const uploadPromises = acceptedFiles.map(uploadFile);

    try {
      await Promise.all(uploadPromises);
      toast.success(`${acceptedFiles.length} file(s) uploaded successfully!`);
      fetchFiles(); // Refresh file list
    } catch (err: any) {
      toast.error(err.response?.data?.error || "An upload failed.");
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    // Confirmation dialog for better UX
    if (!window.confirm("Are you sure you want to delete this file?")) {
      return;
    }

    try {
      await deleteFile(id);
      toast.success("File deleted successfully!");
      setFiles(files.filter((f) => f.id !== id)); // Optimistic UI update
    } catch (err) {
      toast.error("Failed to delete file.");
    }
  };

  // Memoize filtered files to avoid recalculating on every render
  const filteredFiles = useMemo(() =>
    files.filter((f) =>
      f.filename.toLowerCase().includes(search.toLowerCase())
    ), [files, search]
  );

  return (
    <div className="max-w-4xl mx-auto p-6 space-y-6 font-sans">
      <Toaster position="top-center" />
      <header className="text-center">
        <h1 className="text-3xl font-bold text-gray-800">BNID File Vault</h1>
        <p className="text-gray-500">Upload, search, and manage your files with ease.</p>
      </header>

      <Uploader onUpload={handleUpload} loading={loading} />

      <main className="space-y-4">
        <SearchBar value={search} onChange={setSearch} />
        <FileList files={filteredFiles} onDelete={handleDelete} />
      </main>
    </div>
  );
}

export default App;