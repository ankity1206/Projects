#pragma once
// engine/src/core/PdfMerger.h
#include <string>
#include <vector>
#include <functional>

namespace pdfmaster {

struct MergeInput {
    std::string path;
    int fromPage = 1;   // 1-based
    int toPage   = -1;  // -1 = last
};

struct MergeOptions {
    bool linearize         = false;
    bool preserveMetadata  = true;
    bool preserveBookmarks = true;
};

struct MergeResult {
    bool        success      = false;
    std::string errorMessage;
    size_t      outputBytes  = 0;
    int         totalPages   = 0;
    double      elapsedMs    = 0.0;
};

class PdfMerger {
public:
    using ProgressCb = std::function<void(int cur, int total, const std::string& msg)>;

    static MergeResult merge(
        const std::vector<MergeInput>& inputs,
        const std::string&             outputPath,
        const MergeOptions&            opts = {},
        ProgressCb                     cb   = {}
    );

    static MergeResult mergeFiles(
        const std::vector<std::string>& paths,
        const std::string&              outputPath,
        const MergeOptions&             opts = {}
    );
};

} // namespace pdfmaster
