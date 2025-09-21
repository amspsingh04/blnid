// src/components/SearchBar.tsx
import React from 'react';

interface SearchBarProps {
    value: string;
    onChange: (value: string) => void;
}

export function SearchBar({ value, onChange }: SearchBarProps) {
    return (
        <input
            type="text"
            placeholder="Search files..."
            className="w-full border p-2 rounded-lg"
            value={value}
            onChange={(e) => onChange(e.target.value)}
        />
    );
}