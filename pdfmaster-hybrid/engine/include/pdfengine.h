/*
 * engine/include/pdfengine.h
 *
 * PUBLIC C ABI — the only header cgo ever sees.
 *
 * Rules:
 *   - No C++ types, no STL, no exceptions, no RAII.
 *   - All strings in/out are UTF-8, null-terminated.
 *   - Opaque handles are void* to C++ objects; caller must not dereference.
 *   - Functions return PM_OK (0) on success or a negative error code.
 *   - Error strings have static lifetime (never free them).
 *   - Output strings/buffers are heap-allocated by the engine;
 *     caller must call pm_free_string() to release them.
 *   - All functions are thread-safe unless noted.
 */

#ifndef PDFENGINE_H
#define PDFENGINE_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stddef.h>
#include <stdint.h>

/* ── Version ─────────────────────────────────────────────────────── */
#define PM_VERSION_MAJOR 1
#define PM_VERSION_MINOR 0
#define PM_VERSION_PATCH 0
#define PM_VERSION_STR   "1.0.0"

/* ── Error codes ─────────────────────────────────────────────────── */
#define PM_OK                  0
#define PM_ERR_NULL_PTR       -1
#define PM_ERR_FILE_NOT_FOUND -2
#define PM_ERR_FILE_READ      -3
#define PM_ERR_FILE_WRITE     -4
#define PM_ERR_NOT_PDF        -5
#define PM_ERR_CORRUPT        -6
#define PM_ERR_OOM            -7
#define PM_ERR_INVALID_ARG    -8
#define PM_ERR_ENCRYPTED      -9
#define PM_ERR_ZLIB           -10
#define PM_ERR_UNSUPPORTED    -11
#define PM_ERR_ENGINE         -99

/* ── Opaque document handle ──────────────────────────────────────── */
typedef void* pm_doc_t;

/* ── Page info (plain-old-data, safe to cross ABI) ───────────────── */
typedef struct {
    int    page_index;      /* 0-based */
    double width_pt;        /* PDF points (1/72 inch) */
    double height_pt;
    int    rotation;        /* 0 | 90 | 180 | 270 */
    size_t stream_bytes;    /* compressed content size */
} pm_page_info_t;

/* ── Document info ───────────────────────────────────────────────── */
typedef struct {
    char   title[256];
    char   author[256];
    char   subject[256];
    char   creator[256];
    char   producer[256];
    char   creation_date[64];
    char   mod_date[64];
    char   pdf_version[8];
    int    page_count;
    int    encrypted;
    int64_t file_size_bytes;
} pm_doc_info_t;

/* ── Render result (pixel buffer) ────────────────────────────────── */
typedef struct {
    uint8_t* pixels;    /* RGB24, row-major, width*3 bytes/row */
    int      width;
    int      height;
    int      stride;    /* bytes per row (may be padded) */
} pm_render_result_t;

/* ── Compress levels ─────────────────────────────────────────────── */
#define PM_COMPRESS_LIGHT  1
#define PM_COMPRESS_MEDIUM 2
#define PM_COMPRESS_HEAVY  3

/* ── Compress options ────────────────────────────────────────────── */
typedef struct {
    int level;                    /* PM_COMPRESS_* */
    int remove_metadata;          /* 0 or 1 */
    int remove_javascript;        /* 0 or 1 */
    int remove_annotations;       /* 0 or 1 */
    int jpeg_quality;             /* 1-100, used in HEAVY */
    int max_image_dpi;            /* downsample above this DPI */
} pm_compress_opts_t;

/* ── Compress result stats ───────────────────────────────────────── */
typedef struct {
    int64_t original_bytes;
    int64_t output_bytes;
    double  savings_pct;
    int     pages_processed;
    double  elapsed_ms;
} pm_compress_stats_t;

/* ── Merge result stats ──────────────────────────────────────────── */
typedef struct {
    int64_t output_bytes;
    int     total_pages;
    double  elapsed_ms;
} pm_merge_stats_t;

/* ── Split result stats ──────────────────────────────────────────── */
typedef struct {
    int    files_written;
    int    total_pages;
    double elapsed_ms;
} pm_split_stats_t;

/* ── Progress callback ───────────────────────────────────────────── */
/* Called from the engine thread; must be lock-free on the Go side.  */
typedef void (*pm_progress_cb_t)(int current, int total,
                                  const char* message, void* userdata);

/* ════════════════════════════════════════════════════════════════════
 *  ENGINE LIFECYCLE
 * ════════════════════════════════════════════════════════════════════ */

/* Initialise global engine state (call once at startup). */
int pm_engine_init(void);

/* Teardown (call once at shutdown). */
void pm_engine_shutdown(void);

/* Version string — static, never free. */
const char* pm_engine_version(void);

/* Human-readable error string — static, never free. */
const char* pm_strerror(int err_code);

/* Free a heap string returned by the engine (pm_extract_text etc.). */
void pm_free_string(char* s);

/* Free a render result pixel buffer. */
void pm_free_render(pm_render_result_t* r);

/* ════════════════════════════════════════════════════════════════════
 *  DOCUMENT HANDLE API
 * ════════════════════════════════════════════════════════════════════ */

/* Open a PDF. Returns opaque handle or NULL on error.
   err_out receives PM_ERR_* code; may be NULL.
   msg_out receives static error string; may be NULL. */
pm_doc_t pm_doc_open(const char* path,
                      int*        err_out,
                      const char** msg_out);

/* Close and free a document handle. */
void pm_doc_close(pm_doc_t doc);

/* Page count. Returns -1 if doc is NULL. */
int pm_doc_page_count(pm_doc_t doc);

/* Fill pm_doc_info_t for an open document. */
int pm_doc_get_info(pm_doc_t doc, pm_doc_info_t* out);

/* Fill pm_page_info_t for page index (0-based). */
int pm_doc_get_page_info(pm_doc_t doc, int page_index, pm_page_info_t* out);

/* ════════════════════════════════════════════════════════════════════
 *  RENDERING API
 * ════════════════════════════════════════════════════════════════════ */

/* Render one page to an RGB24 pixel buffer.
   dpi: resolution (72 = 1:1 pt, 96 = screen, 150/300 = print).
   rotation_extra: additional rotation in degrees (0|90|180|270).
   out: filled on PM_OK; caller must call pm_free_render(out).
   Thread-safe: each call clones the MuPDF context internally. */
int pm_render_page(pm_doc_t            doc,
                   int                 page_index,
                   float               dpi,
                   int                 rotation_extra,
                   pm_render_result_t* out);

/* ════════════════════════════════════════════════════════════════════
 *  TEXT EXTRACTION API
 * ════════════════════════════════════════════════════════════════════ */

/* Extract UTF-8 text from one page.
   *text_out is heap-allocated; caller must call pm_free_string().
   Returns PM_OK or error code. */
int pm_extract_text_page(pm_doc_t     doc,
                          int          page_index,
                          char**       text_out,
                          size_t*      len_out);

/* Extract text from all pages, concatenated with \f (form feed) separators.
   progress_cb may be NULL. */
int pm_extract_text_all(pm_doc_t         doc,
                         char**           text_out,
                         size_t*          len_out,
                         pm_progress_cb_t progress_cb,
                         void*            userdata);

/* ════════════════════════════════════════════════════════════════════
 *  MERGE API
 * ════════════════════════════════════════════════════════════════════ */

/* Merge multiple PDF files into output_path.
   paths:     array of null-terminated C strings, length n_paths.
   from_pages, to_pages: per-file 1-based page ranges.
              pass NULL for both to include all pages of every file.
   stats_out: may be NULL.
   progress_cb: may be NULL. */
int pm_merge(const char** paths,
             int          n_paths,
             const int*   from_pages,
             const int*   to_pages,
             const char*  output_path,
             int          linearize,
             pm_merge_stats_t*  stats_out,
             pm_progress_cb_t   progress_cb,
             void*              userdata);

/* ════════════════════════════════════════════════════════════════════
 *  COMPRESS API
 * ════════════════════════════════════════════════════════════════════ */

int pm_compress(const char*             input_path,
                const char*             output_path,
                const pm_compress_opts_t* opts,
                pm_compress_stats_t*    stats_out,
                pm_progress_cb_t        progress_cb,
                void*                   userdata);

/* ════════════════════════════════════════════════════════════════════
 *  SPLIT API
 * ════════════════════════════════════════════════════════════════════ */

/* Split modes */
#define PM_SPLIT_PAGES  0   /* one output file per page */
#define PM_SPLIT_RANGE  1   /* extract from_page..to_page into one file */
#define PM_SPLIT_CHUNKS 2   /* sequential groups of chunk_size pages */

int pm_split(const char*       input_path,
             const char*       output_dir,
             const char*       name_template,  /* e.g. "{name}_{from}-{to}.pdf" */
             int               mode,           /* PM_SPLIT_* */
             int               from_page,      /* 1-based, used by RANGE */
             int               to_page,        /* 1-based, -1=last */
             int               chunk_size,     /* used by CHUNKS */
             pm_split_stats_t* stats_out,
             pm_progress_cb_t  progress_cb,
             void*             userdata);

/* ════════════════════════════════════════════════════════════════════
 *  UTILITY
 * ════════════════════════════════════════════════════════════════════ */

/* Quick page count without a full open (stat + xref scan). */
int pm_quick_page_count(const char* path);

/* Validate that a file is a readable PDF (checks header + xref). */
int pm_validate_pdf(const char* path, char** err_msg_out);

#ifdef __cplusplus
}
#endif

#endif /* PDFENGINE_H */
