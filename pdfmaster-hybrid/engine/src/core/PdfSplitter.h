#pragma once
// engine/src/core/PdfSplitter.h
#include <string>
#include <vector>
#include <functional>

namespace pdfmaster {

struct SplitOptions {
    std::string inputPath;
    std::string outputDir;
    std::string nameTemplate;   // "{name}_{from}-{to}.pdf"
    int fromPage  = 1;
    int toPage    = -1;
    int everyN    = 1;          // chunk size; 1 = one file per page
};

struct SplitResult {
    bool                     success = false;
    std::string              errorMessage;
    std::vector<std::string> outputFiles;
    int                      totalPages  = 0;
    double                   elapsedMs   = 0.0;
};

class PdfSplitter {
public:
    using ProgressCb = std::function<void(int, int)>;

    static SplitResult split(
        const SplitOptions& opts,
        ProgressCb          cb = {}
    );

private:
    static std::string formatName(const std::string& tmpl,
                                   const std::string& base,
                                   int chunkIdx, int from, int to);
};

} // namespace pdfmaster
