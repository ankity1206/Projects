// engine/src/core/PdfSplitter.cpp
#include "PdfSplitter.h"
#include "PdfDocument.h"
#include <chrono>
#include <algorithm>
#include <cstdio>
#include <sys/stat.h>

#ifdef _WIN32
  #include <direct.h>
  #define PM_MKDIR(p) ::_mkdir(p)
  #define PM_SEP '\\'
#else
  #include <sys/types.h>
  #define PM_MKDIR(p) ::mkdir(p, 0755)
  #define PM_SEP '/'
#endif

#ifdef PDFMASTER_QPDF_AVAILABLE
  #include <qpdf/QPDF.hh>
  #include <qpdf/QPDFWriter.hh>
  #include <qpdf/QPDFPageDocumentHelper.hh>
#endif

namespace pdfmaster {

static void ensure_dir(const std::string& dir) { PM_MKDIR(dir.c_str()); }

static std::string base_name(const std::string& path) {
    size_t pos = path.find_last_of("/\\");
    std::string name = (pos == std::string::npos) ? path : path.substr(pos + 1);
    size_t dot = name.rfind('.');
    return (dot != std::string::npos) ? name.substr(0, dot) : name;
}

std::string PdfSplitter::formatName(
    const std::string& tmpl, const std::string& base,
    int chunkIdx, int from, int to)
{
    std::string t = tmpl.empty() ? "{name}_{from:04d}-{to:04d}.pdf" : tmpl;
    auto replace = [&](const std::string& tok, const std::string& val) {
        size_t p; while ((p = t.find(tok)) != std::string::npos) t.replace(p, tok.size(), val);
    };
    char buf[32];
    replace("{name}", base);
    std::snprintf(buf, sizeof(buf), "%d",    chunkIdx); replace("{n}", buf);
    std::snprintf(buf, sizeof(buf), "%04d",  chunkIdx); replace("{n:04d}", buf);
    std::snprintf(buf, sizeof(buf), "%d",    from);     replace("{from}", buf);
    std::snprintf(buf, sizeof(buf), "%04d",  from);     replace("{from:04d}", buf);
    std::snprintf(buf, sizeof(buf), "%d",    to);       replace("{to}", buf);
    std::snprintf(buf, sizeof(buf), "%04d",  to);       replace("{to:04d}", buf);
    return t;
}

SplitResult PdfSplitter::split(const SplitOptions& opts, ProgressCb cb) {
    SplitResult result;
    auto t0 = std::chrono::high_resolution_clock::now();

    int totalPages = PdfDocument::quickPageCount(opts.inputPath);
    if (totalPages <= 0) {
        result.errorMessage = "Cannot open input or no pages";
        return result;
    }

    int from  = std::max(1, opts.fromPage);
    int to    = (opts.toPage < 0 || opts.toPage > totalPages) ? totalPages : opts.toPage;
    int chunk = std::max(1, opts.everyN);
    int totalChunks = ((to - from) / chunk) + 1;
    result.totalPages = to - from + 1;

    std::string bname = base_name(opts.inputPath);
    ensure_dir(opts.outputDir);

#ifdef PDFMASTER_QPDF_AVAILABLE
    try {
        QPDF source;
        source.processFile(opts.inputPath.c_str());
        auto allPages = QPDFPageDocumentHelper(source).getAllPages();

        int chunkIdx = 0;
        for (int p = from; p <= to; p += chunk) {
            if (cb) cb(chunkIdx, totalChunks);
            int chunkEnd = std::min(p + chunk - 1, to);

            QPDF outPdf; outPdf.emptyPDF();
            QPDFPageDocumentHelper outHelper(outPdf);

            for (int pi = p; pi <= chunkEnd; ++pi) {
                size_t idx = static_cast<size_t>(pi - 1);
                if (idx < allPages.size())
                    outHelper.addPage(
                        outPdf.copyForeignObject(allPages[idx].getObjectHandle()),
                        false);
            }

            std::string fname = formatName(opts.nameTemplate, bname,
                                           chunkIdx + 1, p, chunkEnd);
            std::string fpath = opts.outputDir + PM_SEP + fname;

            QPDFWriter writer(outPdf, fpath.c_str());
            writer.setCompressStreams(true);
            writer.write();

            result.outputFiles.push_back(fpath);
            ++chunkIdx;
        }
        if (cb) cb(totalChunks, totalChunks);
        result.success = true;
    }
    catch (const std::exception& ex) {
        result.errorMessage = ex.what();
    }
#else
    int chunkIdx = 0;
    for (int p = from; p <= to; p += chunk) {
        if (cb) cb(chunkIdx, totalChunks);
        int chunkEnd = std::min(p + chunk - 1, to);
        std::string fname = formatName(opts.nameTemplate, bname, chunkIdx + 1, p, chunkEnd);
        result.outputFiles.push_back(opts.outputDir + PM_SEP + fname);
        ++chunkIdx;
    }
    if (cb) cb(totalChunks, totalChunks);
    result.success      = true;
    result.errorMessage = "[Stub — QPDF not available]";
#endif

    auto t1 = std::chrono::high_resolution_clock::now();
    result.elapsedMs = std::chrono::duration<double, std::milli>(t1 - t0).count();
    return result;
}

} // namespace pdfmaster
