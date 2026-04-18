#pragma once
// engine/src/core/PdfCompressor.h
#include <string>
#include <functional>

namespace pdfmaster {

enum class CompressionLevel { Light = 1, Medium = 2, Heavy = 3 };

struct CompressOptions {
    CompressionLevel level             = CompressionLevel::Medium;
    bool removeMetadata                = false;
    bool removeJavaScript              = true;
    bool removeAnnotations             = false;
    int  imageJpegQuality              = 72;
    int  imageMaxDpi                   = 150;
};

struct CompressResult {
    bool        success         = false;
    std::string errorMessage;
    size_t      originalBytes   = 0;
    size_t      outputBytes     = 0;
    double      savingsPct      = 0.0;
    int         pagesProcessed  = 0;
    double      elapsedMs       = 0.0;
};

class PdfCompressor {
public:
    using ProgressCb = std::function<void(int, int, const std::string&)>;

    static CompressResult compress(
        const std::string&     inputPath,
        const std::string&     outputPath,
        const CompressOptions& opts = {},
        ProgressCb             cb   = {}
    );
};

} // namespace pdfmaster
