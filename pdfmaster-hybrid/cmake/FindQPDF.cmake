# cmake/FindQPDF.cmake
# Finds QPDF headers and library.
# Imported target: QPDF::QPDF

include(FindPackageHandleStandardArgs)

set(_qpdf_search_roots
    "$ENV{QPDF_ROOT}"
    "${QPDF_ROOT}"
    "${CMAKE_PREFIX_PATH}"
    "/usr/local"
    "/usr"
    "C:/qpdf"
    "C:/vcpkg/installed/x64-windows"
)

find_path(QPDF_INCLUDE_DIR
    NAMES qpdf/QPDF.hh qpdf/QPDFWriter.hh
    HINTS ${_qpdf_search_roots}
    PATH_SUFFIXES include
)

find_library(QPDF_LIBRARY
    NAMES qpdf libqpdf
    HINTS ${_qpdf_search_roots}
    PATH_SUFFIXES lib lib64 lib/x64
)

find_package_handle_standard_args(QPDF
    REQUIRED_VARS QPDF_INCLUDE_DIR QPDF_LIBRARY
)

if(QPDF_FOUND)
    set(QPDF_INCLUDE_DIRS ${QPDF_INCLUDE_DIR})
    set(QPDF_LIBRARIES    ${QPDF_LIBRARY})

    if(NOT TARGET QPDF::QPDF)
        add_library(QPDF::QPDF UNKNOWN IMPORTED)
        set_target_properties(QPDF::QPDF PROPERTIES
            IMPORTED_LOCATION "${QPDF_LIBRARY}"
            INTERFACE_INCLUDE_DIRECTORIES "${QPDF_INCLUDE_DIRS}"
        )
    endif()

    message(STATUS "Found QPDF: ${QPDF_LIBRARY}")
endif()

mark_as_advanced(QPDF_INCLUDE_DIR QPDF_LIBRARY)
