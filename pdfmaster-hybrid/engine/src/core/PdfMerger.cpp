// engine/src/core/PdfMerger.cpp
#include "PdfMerger.h"
#include "PdfDocument.h"

#include <chrono>
#include <stdexcept>
#include <sys/stat.h>

#ifdef PDFMASTER_QPDF_AVAILABLE
  #include <qpdf/QPDF.hh>
  #include <qpdf/QPDFWriter.hh>
  #include <qpdf/QPDFPageDocumentHelper.hh>
  #include <qpdf/QPDFPageObjectHelper.hh>
#endif

namespace pdfmaster {

static size_t file_size(const std::string& path) {
    struct stat st{};
    return (::stat(path.c_str(), &st) == 0) ? static_cast<size_t>(st.st_size) : 0;
}

MergeResult PdfMerger::merge(
    const std::vector<MergeInput>& inputs,
    const std::string&             outputPath,
    const MergeOptions&            opts,
    ProgressCb                     cb)
{
    MergeResult result;
    auto t0 = std::chrono::high_resolution_clock::now();

    if (inputs.empty()) {
        result.errorMessage = "No input files specified";
        return result;
    }

#ifdef PDFMASTER_QPDF_AVAILABLE
    try {
        QPDF output;
        output.emptyPDF();
        QPDFPageDocumentHelper outHelper(output);

        for (size_t fi = 0; fi < inputs.size(); ++fi) {
            if (cb) cb(static_cast<int>(fi), static_cast<int>(inputs.size()), "");

            const auto& inp = inputs[fi];
            QPDF source;
            source.processFile(inp.path.c_str());
            QPDFPageDocumentHelper srcHelper(source);
            auto pages = srcHelper.getAllPages();
            int total  = static_cast<int>(pages.size());
            int from   = std::max(0, inp.fromPage - 1);
            int to     = (inp.toPage < 0) ? total - 1 : std::min(inp.toPage - 1, total - 1);

            for (int pi = from; pi <= to; ++pi) {
                outHelper.addPage(
                    output.copyForeignObject(
                        pages[static_cast<size_t>(pi)].getObjectHandle()),
                    false);
                result.totalPages++;
            }

            /* Copy metadata from first file */
            if (fi == 0 && opts.preserveMetadata) {
                QPDFObjectHandle info = source.getTrailer().getKey("/Info");
                if (!info.isNull())
                    output.getTrailer().replaceKey("/Info",
                        output.copyForeignObject(info));
            }
        }

        if (cb) cb(static_cast<int>(inputs.size()),
                   static_cast<int>(inputs.size()), "Writing output");

        QPDFWriter writer(output, outputPath.c_str());
        writer.setStreamDataMode(qpdf_s_compress);
        writer.setCompressStreams(true);
        writer.setRecompressFlate(false);
        if (opts.linearize) writer.setLinearization(true);
        writer.write();

        result.outputBytes = file_size(outputPath);
        result.success     = true;
    }
    catch (const std::exception& ex) {
        result.errorMessage = ex.what();
    }
#else
    /* Stub */
    (void)opts; (void)cb;
    for (const auto& inp : inputs)
        result.totalPages += PdfDocument::quickPageCount(inp.path);
    result.success      = true;
    result.errorMessage = "[Stub — QPDF not available]";
#endif

    auto t1 = std::chrono::high_resolution_clock::now();
    result.elapsedMs = std::chrono::duration<double, std::milli>(t1 - t0).count();
    return result;
}

MergeResult PdfMerger::mergeFiles(
    const std::vector<std::string>& paths,
    const std::string&              outputPath,
    const MergeOptions&             opts)
{
    std::vector<MergeInput> inputs;
    inputs.reserve(paths.size());
    for (const auto& p : paths) { MergeInput mi; mi.path = p; inputs.push_back(mi); }
    return merge(inputs, outputPath, opts);
}

} // namespace pdfmaster
