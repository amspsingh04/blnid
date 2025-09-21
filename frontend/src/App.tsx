import React, { useState, useEffect } from "react";
import axios from "axios";

function App() {
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState("");
  const [files, setFiles] = useState<any[]>([]);

  const fetchFiles = async () => {
    try {
      const res = await axios.get("http://localhost:8080/files");
      console.log(res.data); // Check what backend returns
      setFiles(res.data);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    fetchFiles();
  }, []);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) setFile(e.target.files[0]);
  };

  const handleUpload = async () => {
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);

    try {
      const res = await axios.post("http://localhost:8080/upload", formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      setMessage(res.data.message);
      fetchFiles(); // Refresh file list after upload
    } catch (err: any) {
      setMessage(err.response?.data?.error || "Upload failed");
    }
  };

  return (
    <div style={{ padding: "2rem" }}>
      <h2>BNID File Vault Upload</h2>
      <input type="file" onChange={handleFileChange} />
      <button onClick={handleUpload}>Upload</button>
      <p>{message}</p>

      <h3>Uploaded Files</h3>
      <ul>
        {files.map((f) => (
          <li key={f.id}>
            {f.filename} ({f.mime_type}, {f.size} bytes)
          </li>
        ))}
      </ul>
    </div>
  );
}

export default App;
