// engine/src/core/PdfCompressor.cpp
#include "PdfCompressor.h"
#include <chrono>
#include <stdexcept>
#include <sys/stat.h>
#include <cstdio>

#ifdef PDFMASTER_QPDF_AVAILABLE
  #include <qpdf/QPDF.hh>
  #include <qpdf/QPDFWriter.hh>
  #include <qpdf/QPDFPageDocumentHelper.hh>
#endif

namespace pdfmaster {

static size_t file_size_c(const std::string& p) {
    struct stat st{}; return (::stat(p.c_str(), &st) == 0) ? (size_t)st.st_size : 0;
}

CompressResult PdfCompressor::compress(
    const std::string&     inputPath,
    const std::string&     outputPath,
    const CompressOptions& opts,
    ProgressCb             cb)
{
    CompressResult result;
    auto t0 = std::chrono::high_resolution_clock::now();

    result.originalBytes = file_size_c(inputPath);
    if (result.originalBytes == 0) {
        result.errorMessage = "Input file not found or empty";
        return result;
    }

    if (cb) cb(0, 10, "Opening document");

#ifdef PDFMASTER_QPDF_AVAILABLE
    try {
        QPDF pdf;
        pdf.processFile(inputPath.c_str());
        result.pagesProcessed = QPDFPageDocumentHelper(pdf).getAllPages().size();

        if (cb) cb(2, 10, "Analysing streams");

        /* Strip optional content */
        if (opts.removeMetadata) {
            pdf.getTrailer().replaceKey("/Info", QPDFObjectHandle::newNull());
            auto catalog = pdf.getRoot();
            if (catalog.hasKey("/Metadata"))
                catalog.replaceKey("/Metadata", QPDFObjectHandle::newNull());
        }
        if (opts.removeJavaScript) {
            for (auto& obj : pdf.getAllObjects()) {
                if (!obj.isDictionary()) continue;
                auto s = obj.getKey("/S");
                if (s.isName() && s.getName() == "/JavaScript")
                    obj.replaceKey("/JS", QPDFObjectHandle::newNull());
            }
        }

        if (cb) cb(4, 10, "Configuring writer");

        QPDFWriter writer(pdf, outputPath.c_str());
        writer.setObjectStreamMode(qpdf_o_generate);
        writer.setCompressStreams(true);

        if (opts.level >= CompressionLevel::Medium) {
            writer.setRecompressFlate(true);
            writer.setDecodeLevel(qpdf_dl_generalized);
        }
        if (opts.level >= CompressionLevel::Heavy) {
            writer.setDecodeLevel(qpdf_dl_all);
            writer.setRecompressFlate(true);
        }

        writer.setLinearization(false);

        if (cb) cb(7, 10, "Writing compressed output");
        writer.write();

        result.outputBytes  = file_size_c(outputPath);
        result.savingsPct   = result.originalBytes > 0
            ? (1.0 - (double)result.outputBytes / (double)result.originalBytes) * 100.0
            : 0.0;
        result.success = true;
        if (cb) cb(10, 10, "Done");
    }
    catch (const std::exception& ex) {
        result.errorMessage = ex.what();
    }
#else
    /* Stub: copy input to output */
    if (cb) cb(5, 10, "Copying (stub)");
    FILE* in  = std::fopen(inputPath.c_str(), "rb");
    FILE* out = std::fopen(outputPath.c_str(), "wb");
    if (in && out) {
        char buf[65536]; size_t n;
        while ((n = std::fread(buf, 1, sizeof(buf), in)) > 0) std::fwrite(buf, 1, n, out);
    }
    if (in)  std::fclose(in);
    if (out) std::fclose(out);
    result.outputBytes  = result.originalBytes;
    result.savingsPct   = 0.0;
    result.success      = true;
    result.errorMessage = "[Stub — QPDF not available]";
    if (cb) cb(10, 10, "Done (stub)");
#endif

    auto t1 = std::chrono::high_resolution_clock::now();
    result.elapsedMs = std::chrono::duration<double, std::milli>(t1 - t0).count();
    return result;
}

} // namespace pdfmaster
