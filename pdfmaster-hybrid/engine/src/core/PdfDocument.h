#pragma once
// engine/src/core/PdfDocument.h
// Pure C++ / std:: — zero Qt dependency in the engine layer.

#include <string>
#include <vector>
#include <functional>
#include <cstdint>
#include <cstddef>

// Forward-declare MuPDF types
struct fz_context_s;
struct fz_document_s;

namespace pdfmaster {

/* ── Geometry ─────────────────────────────────────────────────────── */
struct SizeD { double x = 595.0; double y = 842.0; };

/* ── Page metadata ────────────────────────────────────────────────── */
struct PageInfo {
    int    index      = 0;
    SizeD  sizePt;
    int    rotation   = 0;
    size_t streamBytes= 0;
};

/* ── Document metadata ────────────────────────────────────────────── */
struct DocInfo {
    std::string title, author, subject, creator, producer;
    std::string creationDate, modDate, pdfVersion;
    int    pageCount      = 0;
    bool   isEncrypted    = false;
    size_t fileSizeBytes  = 0;
};

/* ── Render options ───────────────────────────────────────────────── */
struct RenderOptions {
    float dpi        = 96.0f;
    bool  antialias  = true;
    int   colorspace = 0;  // 0=RGB, 1=Gray
};

/* ── Raw RGB image ────────────────────────────────────────────────── */
struct RgbImage {
    std::vector<uint8_t> pixels;  // RGB24, row-major
    int width  = 0;
    int height = 0;
    int stride = 0;               // bytes per row
};

/* ── PdfDocument ──────────────────────────────────────────────────── */
class PdfDocument {
public:
    PdfDocument();
    ~PdfDocument();

    PdfDocument(const PdfDocument&)            = delete;
    PdfDocument& operator=(const PdfDocument&) = delete;

    bool open(const std::string& path, std::string* errOut = nullptr);
    void close();
    bool isOpen() const;

    int      pageCount() const;
    DocInfo  docInfo()   const;
    PageInfo pageInfo(int pageIndex) const;
    std::string filePath() const { return m_filePath; }

    /* renderPage: thread-safe (clones the MuPDF context per call) */
    RgbImage renderPage(int pageIndex,
                        const RenderOptions& opts    = {},
                        int rotationExtra            = 0) const;

    std::string extractText(int pageIndex) const;

    /* Static utility — opens a fresh doc, counts pages, closes */
    static int quickPageCount(const std::string& path);

private:
    fz_context_s*  m_ctx       = nullptr;
    fz_document_s* m_doc       = nullptr;
    std::string    m_filePath;
    int            m_pageCount = 0;
};

} // namespace pdfmaster
