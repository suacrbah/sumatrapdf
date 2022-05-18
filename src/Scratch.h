/* Copyright 2022 the SumatraPDF project authors (see AUTHORS file).
   License: Simplified BSD (see COPYING.BSD) */

struct FileData {
    char* name = nullptr;
    ByteSlice data{};
    int imageNo = 0; // counting from 1

    FileData() = default;
    ~FileData() {
        str::Free(name);
        str::Free(data);
    }
};

Vec<FileData*> MobiToEpub(const char* path);

void LoadRar();
void LoadFile();
