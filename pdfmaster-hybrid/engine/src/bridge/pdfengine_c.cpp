/*
 * engine/src/bridge/pdfengine_c.cpp
 *
 * The C ABI bridge — the ONLY translation unit that exports extern "C"
 * symbols.  All C++ types stop here: only POD structs and primitive
 * types cross into the header that cgo reads.
 *
 * Error handling philosophy:
 *   - C++ exceptions are caught here and turned into PM_ERR_* codes.
 *   - Never let an exception propagate across the C boundary.
 *   - Never return a pointer to a stack variable.
 */

#include "pdfengine.h"

/* C++ engine headers */
#include "../core/PdfDocument.h"
#include "../core/PdfMerger.h"
#include "../core/PdfCompressor.h"
#include "../core/PdfSplitter.h"

#include <cstring>
#include <cstdlib>
#include <cstdio>
#include <cassert>
#include <stdexcept>
#include <string>
#include <vector>

/* ── Internal helpers ─────────────────────────────────────────────── */

static char* heap_strdup(const std::string& s) {
    char* p = static_cast<char*>(std::malloc(s.size() + 1));
    if (p) std::memcpy(p, s.c_str(), s.size() + 1);
    return p;
}

static void copy_str(char* dst, size_t dstsz, const std::string& src) {
    size_t n = src.size() < dstsz - 1 ? src.size() : dstsz - 1;
    std::memcpy(dst, src.c_str(), n);
    dst[n] = '\0';
}

/* Wrap a C++ engine call; catch everything and return an error code. */
#define PM_TRY(expr)                        \
    do {                                    \
        try { expr; }                       \
        catch (const std::bad_alloc&) {     \
            return PM_ERR_OOM;              \
        }                                   \
        catch (const std::exception& _e) {  \
            (void)_e;                       \
            return PM_ERR_ENGINE;           \
        }                                   \
        catch (...) {                       \
            return PM_ERR_ENGINE;           \
        }                                   \
    } while (0)

/* ════════════════════════════════════════════════════════════════════
 *  ENGINE LIFECYCLE
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_engine_init(void) {
    /* Nothing global to initialise in this build; each document gets
       its own MuPDF context. Reserved for future thread-pool setup. */
    return PM_OK;
}

extern "C" void pm_engine_shutdown(void) {
    /* Reserved for future teardown. */
}

extern "C" const char* pm_engine_version(void) {
    return PM_VERSION_STR;
}

extern "C" const char* pm_strerror(int code) {
    switch (code) {
        case PM_OK:                return "OK";
        case PM_ERR_NULL_PTR:      return "Null pointer";
        case PM_ERR_FILE_NOT_FOUND:return "File not found";
        case PM_ERR_FILE_READ:     return "File read error";
        case PM_ERR_FILE_WRITE:    return "File write error";
        case PM_ERR_NOT_PDF:       return "Not a valid PDF";
        case PM_ERR_CORRUPT:       return "Corrupt or malformed PDF";
        case PM_ERR_OOM:           return "Out of memory";
        case PM_ERR_INVALID_ARG:   return "Invalid argument";
        case PM_ERR_ENCRYPTED:     return "Encrypted PDF";
        case PM_ERR_ZLIB:          return "zlib error";
        case PM_ERR_UNSUPPORTED:   return "Unsupported feature";
        case PM_ERR_ENGINE:        return "Internal engine error";
        default:                   return "Unknown error";
    }
}

extern "C" void pm_free_string(char* s) {
    std::free(s);
}

extern "C" void pm_free_render(pm_render_result_t* r) {
    if (r && r->pixels) {
        std::free(r->pixels);
        r->pixels = nullptr;
        r->width = r->height = r->stride = 0;
    }
}

/* ════════════════════════════════════════════════════════════════════
 *  DOCUMENT HANDLE API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" pm_doc_t pm_doc_open(const char*  path,
                                 int*         err_out,
                                 const char** msg_out)
{
    if (!path) {
        if (err_out) *err_out = PM_ERR_NULL_PTR;
        if (msg_out) *msg_out = pm_strerror(PM_ERR_NULL_PTR);
        return nullptr;
    }

    pdfmaster::PdfDocument* doc = nullptr;
    try {
        doc = new pdfmaster::PdfDocument();
        std::string err;
        if (!doc->open(std::string(path), &err)) {
            int code = PM_ERR_FILE_NOT_FOUND;
            if (err.find("not a PDF") != std::string::npos
             || err.find("not valid") != std::string::npos)
                code = PM_ERR_NOT_PDF;
            else if (err.find("ncrypt") != std::string::npos)
                code = PM_ERR_ENCRYPTED;
            else if (err.find("corrupt") != std::string::npos)
                code = PM_ERR_CORRUPT;
            if (err_out) *err_out = code;
            if (msg_out) *msg_out = pm_strerror(code);
            delete doc;
            return nullptr;
        }
    } catch (const std::bad_alloc&) {
        if (err_out) *err_out = PM_ERR_OOM;
        if (msg_out) *msg_out = pm_strerror(PM_ERR_OOM);
        delete doc;
        return nullptr;
    } catch (const std::exception& e) {
        if (err_out) *err_out = PM_ERR_ENGINE;
        if (msg_out) *msg_out = pm_strerror(PM_ERR_ENGINE);
        delete doc;
        return nullptr;
    }

    if (err_out) *err_out = PM_OK;
    if (msg_out) *msg_out = "";
    return static_cast<pm_doc_t>(doc);
}

extern "C" void pm_doc_close(pm_doc_t handle) {
    if (!handle) return;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);
    doc->close();
    delete doc;
}

extern "C" int pm_doc_page_count(pm_doc_t handle) {
    if (!handle) return -1;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);
    return doc->pageCount();
}

extern "C" int pm_doc_get_info(pm_doc_t handle, pm_doc_info_t* out) {
    if (!handle || !out) return PM_ERR_NULL_PTR;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);

    PM_TRY({
        pdfmaster::DocInfo info = doc->docInfo();
        copy_str(out->title,         sizeof(out->title),         info.title);
        copy_str(out->author,        sizeof(out->author),        info.author);
        copy_str(out->subject,       sizeof(out->subject),       info.subject);
        copy_str(out->creator,       sizeof(out->creator),       info.creator);
        copy_str(out->producer,      sizeof(out->producer),      info.producer);
        copy_str(out->creation_date, sizeof(out->creation_date), info.creationDate);
        copy_str(out->mod_date,      sizeof(out->mod_date),      info.modDate);
        copy_str(out->pdf_version,   sizeof(out->pdf_version),   info.pdfVersion);
        out->page_count      = info.pageCount;
        out->encrypted       = info.isEncrypted ? 1 : 0;
        out->file_size_bytes = static_cast<int64_t>(info.fileSizeBytes);
    });
    return PM_OK;
}

extern "C" int pm_doc_get_page_info(pm_doc_t       handle,
                                     int            page_index,
                                     pm_page_info_t* out)
{
    if (!handle || !out) return PM_ERR_NULL_PTR;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);

    PM_TRY({
        pdfmaster::PageInfo pi = doc->pageInfo(page_index);
        out->page_index   = pi.index;
        out->width_pt     = pi.sizePt.x;
        out->height_pt    = pi.sizePt.y;
        out->rotation     = pi.rotation;
        out->stream_bytes = pi.streamBytes;
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  RENDERING API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_render_page(pm_doc_t            handle,
                               int                 page_index,
                               float               dpi,
                               int                 rotation_extra,
                               pm_render_result_t* out)
{
    if (!handle || !out) return PM_ERR_NULL_PTR;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);

    PM_TRY({
        pdfmaster::RenderOptions opts;
        opts.dpi = dpi > 0 ? dpi : 96.0f;

        pdfmaster::RgbImage img = doc->renderPage(page_index, opts, rotation_extra);
        if (img.pixels.empty()) return PM_ERR_ENGINE;

        size_t sz = img.pixels.size();
        out->pixels = static_cast<uint8_t*>(std::malloc(sz));
        if (!out->pixels) return PM_ERR_OOM;
        std::memcpy(out->pixels, img.pixels.data(), sz);
        out->width  = img.width;
        out->height = img.height;
        out->stride = img.stride;
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  TEXT EXTRACTION API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_extract_text_page(pm_doc_t  handle,
                                     int       page_index,
                                     char**    text_out,
                                     size_t*   len_out)
{
    if (!handle || !text_out) return PM_ERR_NULL_PTR;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);

    PM_TRY({
        std::string text = doc->extractText(page_index);
        *text_out = heap_strdup(text);
        if (!*text_out) return PM_ERR_OOM;
        if (len_out) *len_out = text.size();
    });
    return PM_OK;
}

extern "C" int pm_extract_text_all(pm_doc_t         handle,
                                    char**           text_out,
                                    size_t*          len_out,
                                    pm_progress_cb_t progress_cb,
                                    void*            userdata)
{
    if (!handle || !text_out) return PM_ERR_NULL_PTR;
    auto* doc = static_cast<pdfmaster::PdfDocument*>(handle);

    PM_TRY({
        std::string all;
        int pages = doc->pageCount();
        for (int i = 0; i < pages; ++i) {
            if (i > 0) all += '\f';
            all += doc->extractText(i);
            if (progress_cb) progress_cb(i + 1, pages, "Extracting text", userdata);
        }
        *text_out = heap_strdup(all);
        if (!*text_out) return PM_ERR_OOM;
        if (len_out) *len_out = all.size();
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  MERGE API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_merge(const char**      paths,
                         int               n_paths,
                         const int*        from_pages,
                         const int*        to_pages,
                         const char*       output_path,
                         int               linearize,
                         pm_merge_stats_t* stats_out,
                         pm_progress_cb_t  progress_cb,
                         void*             userdata)
{
    if (!paths || n_paths <= 0 || !output_path) return PM_ERR_INVALID_ARG;

    PM_TRY({
        std::vector<pdfmaster::MergeInput> inputs;
        inputs.reserve(static_cast<size_t>(n_paths));
        for (int i = 0; i < n_paths; ++i) {
            pdfmaster::MergeInput mi;
            mi.path     = paths[i] ? paths[i] : "";
            mi.fromPage = from_pages ? from_pages[i] : 1;
            mi.toPage   = to_pages   ? to_pages[i]   : -1;
            inputs.push_back(mi);
        }

        pdfmaster::MergeOptions opts;
        opts.linearize = linearize != 0;

        auto cb = [&](int cur, int total, const std::string& msg) {
            if (progress_cb)
                progress_cb(cur, total, msg.c_str(), userdata);
        };

        pdfmaster::MergeResult r =
            pdfmaster::PdfMerger::merge(inputs, std::string(output_path), opts, cb);

        if (!r.success) return PM_ERR_ENGINE;

        if (stats_out) {
            stats_out->output_bytes = static_cast<int64_t>(r.outputBytes);
            stats_out->total_pages  = r.totalPages;
            stats_out->elapsed_ms   = r.elapsedMs;
        }
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  COMPRESS API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_compress(const char*               input_path,
                            const char*               output_path,
                            const pm_compress_opts_t* opts,
                            pm_compress_stats_t*      stats_out,
                            pm_progress_cb_t          progress_cb,
                            void*                     userdata)
{
    if (!input_path || !output_path) return PM_ERR_INVALID_ARG;

    PM_TRY({
        pdfmaster::CompressOptions copts;
        if (opts) {
            copts.level = static_cast<pdfmaster::CompressionLevel>(
                opts->level >= 1 && opts->level <= 3 ? opts->level : 2);
            copts.removeMetadata   = opts->remove_metadata != 0;
            copts.removeJavaScript = opts->remove_javascript != 0;
            copts.removeAnnotations= opts->remove_annotations != 0;
            copts.imageJpegQuality = opts->jpeg_quality > 0 ? opts->jpeg_quality : 72;
            copts.imageMaxDpi      = opts->max_image_dpi > 0 ? opts->max_image_dpi : 150;
        }

        auto cb = [&](int step, int total, const std::string& msg) {
            if (progress_cb)
                progress_cb(step, total, msg.c_str(), userdata);
        };

        pdfmaster::CompressResult r =
            pdfmaster::PdfCompressor::compress(
                std::string(input_path),
                std::string(output_path),
                copts, cb);

        if (!r.success) return PM_ERR_ENGINE;

        if (stats_out) {
            stats_out->original_bytes  = static_cast<int64_t>(r.originalBytes);
            stats_out->output_bytes    = static_cast<int64_t>(r.outputBytes);
            stats_out->savings_pct     = r.savingsPct;
            stats_out->pages_processed = r.pagesProcessed;
            stats_out->elapsed_ms      = r.elapsedMs;
        }
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  SPLIT API
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_split(const char*       input_path,
                         const char*       output_dir,
                         const char*       name_template,
                         int               mode,
                         int               from_page,
                         int               to_page,
                         int               chunk_size,
                         pm_split_stats_t* stats_out,
                         pm_progress_cb_t  progress_cb,
                         void*             userdata)
{
    if (!input_path || !output_dir) return PM_ERR_INVALID_ARG;

    PM_TRY({
        pdfmaster::SplitOptions sopts;
        sopts.inputPath     = std::string(input_path);
        sopts.outputDir     = std::string(output_dir);
        sopts.nameTemplate  = name_template ? std::string(name_template) : "";
        sopts.fromPage      = from_page > 0 ? from_page : 1;
        sopts.toPage        = to_page;
        sopts.everyN        = chunk_size > 0 ? chunk_size : 1;

        switch (mode) {
            case PM_SPLIT_RANGE:  sopts.everyN = (to_page - from_page + 1); break;
            case PM_SPLIT_CHUNKS: break; /* everyN already set */
            default:              sopts.everyN = 1; break; /* per-page */
        }

        auto cb = [&](int cur, int total) {
            if (progress_cb)
                progress_cb(cur, total, "Splitting", userdata);
        };

        pdfmaster::SplitResult r =
            pdfmaster::PdfSplitter::split(sopts, cb);

        if (!r.success) return PM_ERR_ENGINE;

        if (stats_out) {
            stats_out->files_written = static_cast<int>(r.outputFiles.size());
            stats_out->total_pages   = r.totalPages;
            stats_out->elapsed_ms    = r.elapsedMs;
        }
    });
    return PM_OK;
}

/* ════════════════════════════════════════════════════════════════════
 *  UTILITY
 * ════════════════════════════════════════════════════════════════════ */

extern "C" int pm_quick_page_count(const char* path) {
    if (!path) return PM_ERR_NULL_PTR;
    PM_TRY({
        return pdfmaster::PdfDocument::quickPageCount(std::string(path));
    });
    return PM_ERR_ENGINE;
}

extern "C" int pm_validate_pdf(const char* path, char** err_msg_out) {
    if (!path) return PM_ERR_NULL_PTR;
    PM_TRY({
        pdfmaster::PdfDocument doc;
        std::string err;
        if (!doc.open(std::string(path), &err)) {
            if (err_msg_out) *err_msg_out = heap_strdup(err);
            return PM_ERR_NOT_PDF;
        }
        doc.close();
        if (err_msg_out) *err_msg_out = nullptr;
    });
    return PM_OK;
}
