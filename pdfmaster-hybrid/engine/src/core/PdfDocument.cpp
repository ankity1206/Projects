// engine/src/core/PdfDocument.cpp
#include "PdfDocument.h"

#include <cstring>
#include <cstdlib>
#include <algorithm>
#include <sys/stat.h>

#ifdef PDFMASTER_MUPDF_AVAILABLE
extern "C" {
  #include <mupdf/fitz.h>
  #include <mupdf/pdf.h>
}
#endif

namespace pdfmaster {

/* ── Constructor / Destructor ─────────────────────────────────────── */
PdfDocument::PdfDocument()  = default;
PdfDocument::~PdfDocument() { close(); }

/* ── open ─────────────────────────────────────────────────────────── */
bool PdfDocument::open(const std::string& path, std::string* errOut) {
    close();
    m_filePath = path;

#ifdef PDFMASTER_MUPDF_AVAILABLE
    m_ctx = fz_new_context(nullptr, nullptr, FZ_STORE_DEFAULT);
    if (!m_ctx) {
        if (errOut) *errOut = "Failed to create MuPDF context";
        return false;
    }
    fz_try(m_ctx) {
        fz_register_document_handlers(m_ctx);
        m_doc = fz_open_document(m_ctx, path.c_str());
        m_pageCount = fz_count_pages(m_ctx, m_doc);
    }
    fz_catch(m_ctx) {
        if (errOut) *errOut = std::string(fz_caught_message(m_ctx));
        fz_drop_context(m_ctx); m_ctx = nullptr;
        return false;
    }
    return true;
#else
    /* Stub: validate PDF header */
    FILE* f = std::fopen(path.c_str(), "rb");
    if (!f) {
        if (errOut) *errOut = "Cannot open file: " + path;
        return false;
    }
    char hdr[6] = {};
    std::fread(hdr, 1, 5, f);
    std::fclose(f);
    if (std::memcmp(hdr, "%PDF-", 5) != 0) {
        if (errOut) *errOut = "Not a valid PDF file";
        return false;
    }
    m_pageCount = 1; /* stub */
    return true;
#endif
}

/* ── close ────────────────────────────────────────────────────────── */
void PdfDocument::close() {
#ifdef PDFMASTER_MUPDF_AVAILABLE
    if (m_doc && m_ctx) { fz_drop_document(m_ctx, m_doc); m_doc = nullptr; }
    if (m_ctx)          { fz_drop_context(m_ctx);          m_ctx = nullptr; }
#endif
    m_pageCount = 0;
}

bool PdfDocument::isOpen() const { return m_doc != nullptr || m_pageCount > 0; }
int  PdfDocument::pageCount() const { return m_pageCount; }

/* ── docInfo ──────────────────────────────────────────────────────── */
DocInfo PdfDocument::docInfo() const {
    DocInfo info;
    info.pageCount = m_pageCount;

    /* File size */
    struct stat st{};
    if (::stat(m_filePath.c_str(), &st) == 0)
        info.fileSizeBytes = static_cast<size_t>(st.st_size);

#ifdef PDFMASTER_MUPDF_AVAILABLE
    if (!m_doc || !m_ctx) return info;

    auto meta = [&](const char* key) -> std::string {
        char buf[512] = {};
        fz_try(m_ctx) { fz_lookup_metadata(m_ctx, m_doc, key, buf, sizeof(buf)); }
        fz_catch(m_ctx) {}
        return std::string(buf);
    };

    info.title        = meta("info:Title");
    info.author       = meta("info:Author");
    info.subject      = meta("info:Subject");
    info.creator      = meta("info:Creator");
    info.producer     = meta("info:Producer");
    info.creationDate = meta("info:CreationDate");
    info.modDate      = meta("info:ModDate");

    /* Version + encryption */
    pdf_document* pdoc = pdf_specifics(m_ctx, m_doc);
    if (pdoc) {
        int major = 0, minor = 0;
        pdf_get_version(m_ctx, pdoc, &major, &minor);
        info.pdfVersion = std::to_string(major) + "." + std::to_string(minor);
        info.isEncrypted = (pdf_needs_password(m_ctx, m_doc) != 0);
    }
#endif
    return info;
}

/* ── pageInfo ─────────────────────────────────────────────────────── */
PageInfo PdfDocument::pageInfo(int idx) const {
    PageInfo pi; pi.index = idx;
#ifdef PDFMASTER_MUPDF_AVAILABLE
    if (!m_doc || !m_ctx || idx < 0 || idx >= m_pageCount) return pi;
    fz_try(m_ctx) {
        fz_rect b = fz_bound_page_number(m_ctx, m_doc, idx);
        pi.sizePt = { std::abs(static_cast<double>(b.x1 - b.x0)),
                      std::abs(static_cast<double>(b.y1 - b.y0)) };
    }
    fz_catch(m_ctx) {}
#endif
    return pi;
}

/* ── renderPage ───────────────────────────────────────────────────── */
RgbImage PdfDocument::renderPage(int pageIndex,
                                  const RenderOptions& opts,
                                  int rotationExtra) const
{
    RgbImage result;

#ifdef PDFMASTER_MUPDF_AVAILABLE
    if (!m_doc || !m_ctx || pageIndex < 0 || pageIndex >= m_pageCount)
        return result;

    /* Clone context so this function is thread-safe */
    fz_context* rctx = fz_clone_context(m_ctx);
    if (!rctx) return result;

    fz_pixmap* pix = nullptr;
    fz_try(rctx) {
        fz_register_document_handlers(rctx);
        fz_document* rdoc = fz_open_document(rctx, m_filePath.c_str());

        float scale = opts.dpi / 72.0f;
        fz_matrix ctm = fz_scale(scale, scale);
        if (rotationExtra != 0)
            ctm = fz_concat(ctm, fz_rotate(static_cast<float>(rotationExtra)));

        fz_colorspace* cs = (opts.colorspace == 1)
            ? fz_device_gray(rctx) : fz_device_rgb(rctx);

        pix = fz_new_pixmap_from_page_number(rctx, rdoc, pageIndex, ctm, cs, 0);

        result.width  = pix->w;
        result.height = pix->h;
        result.stride = pix->stride;
        size_t sz = static_cast<size_t>(pix->h) * static_cast<size_t>(pix->stride);
        result.pixels.resize(sz);
        std::memcpy(result.pixels.data(), pix->samples, sz);

        fz_drop_pixmap(rctx, pix); pix = nullptr;
        fz_drop_document(rctx, rdoc);
    }
    fz_catch(rctx) {
        if (pix) fz_drop_pixmap(rctx, pix);
        result = {};
    }
    fz_drop_context(rctx);
#else
    /* Stub: return a white page */
    (void)opts; (void)rotationExtra;
    result.width  = 595;
    result.height = 842;
    result.stride = 595 * 3;
    result.pixels.assign(static_cast<size_t>(595 * 842 * 3), 255);
#endif

    return result;
}

/* ── extractText ──────────────────────────────────────────────────── */
std::string PdfDocument::extractText(int pageIndex) const {
#ifdef PDFMASTER_MUPDF_AVAILABLE
    if (!m_doc || !m_ctx || pageIndex < 0 || pageIndex >= m_pageCount) return {};
    std::string text;
    fz_try(m_ctx) {
        fz_page* page = fz_load_page(m_ctx, m_doc, pageIndex);
        fz_stext_page* stp = fz_new_stext_page_from_page(m_ctx, page, nullptr);
        fz_buffer* buf = fz_new_buffer_from_stext_page(m_ctx, stp);
        size_t len = 0;
        const char* raw = reinterpret_cast<const char*>(
            fz_buffer_storage(m_ctx, buf, reinterpret_cast<size_t*>(&len)));
        if (raw) text.assign(raw, len);
        fz_drop_buffer(m_ctx, buf);
        fz_drop_stext_page(m_ctx, stp);
        fz_drop_page(m_ctx, page);
    }
    fz_catch(m_ctx) {}
    return text;
#else
    (void)pageIndex;
    return "[MuPDF not available]";
#endif
}

/* ── quickPageCount ───────────────────────────────────────────────── */
int PdfDocument::quickPageCount(const std::string& path) {
    PdfDocument d;
    if (!d.open(path)) return -1;
    return d.pageCount();
}

} // namespace pdfmaster
