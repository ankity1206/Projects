# cmake/FindMuPDF.cmake
# Finds MuPDF headers and libraries.
#
# Search order:
#   1. MUPDF_ROOT env / cmake var
#   2. vcpkg toolchain paths
#   3. Standard system paths
#
# Imported target: MuPDF::MuPDF

include(FindPackageHandleStandardArgs)

set(_mupdf_search_roots
    "$ENV{MUPDF_ROOT}"
    "${MUPDF_ROOT}"
    "${CMAKE_PREFIX_PATH}"
    "/usr/local"
    "/usr"
    "C:/mupdf"
    "C:/vcpkg/installed/x64-windows"
    "C:/vcpkg/installed/x86-windows"
)

find_path(MuPDF_INCLUDE_DIR
    NAMES mupdf/fitz.h mupdf/pdf.h
    HINTS ${_mupdf_search_roots}
    PATH_SUFFIXES include
)

# MuPDF may be split into multiple libs or a single combined lib
find_library(MuPDF_LIBRARY
    NAMES mupdf libmupdf
    HINTS ${_mupdf_search_roots}
    PATH_SUFFIXES lib lib64 lib/x64 lib/x86
)

find_library(MuPDF_THIRD_LIBRARY
    NAMES mupdf-third libmupdf-third mupdf_third
    HINTS ${_mupdf_search_roots}
    PATH_SUFFIXES lib lib64 lib/x64 lib/x86
)

find_package_handle_standard_args(MuPDF
    REQUIRED_VARS MuPDF_INCLUDE_DIR MuPDF_LIBRARY
    VERSION_VAR MuPDF_VERSION
)

if(MuPDF_FOUND)
    set(MuPDF_INCLUDE_DIRS ${MuPDF_INCLUDE_DIR})
    if(MuPDF_THIRD_LIBRARY)
        set(MuPDF_LIBRARIES ${MuPDF_LIBRARY} ${MuPDF_THIRD_LIBRARY})
    else()
        set(MuPDF_LIBRARIES ${MuPDF_LIBRARY})
    endif()

    if(NOT TARGET MuPDF::MuPDF)
        add_library(MuPDF::MuPDF UNKNOWN IMPORTED)
        set_target_properties(MuPDF::MuPDF PROPERTIES
            IMPORTED_LOCATION "${MuPDF_LIBRARY}"
            INTERFACE_INCLUDE_DIRECTORIES "${MuPDF_INCLUDE_DIRS}"
        )
    endif()

    message(STATUS "Found MuPDF: ${MuPDF_LIBRARY}")
    message(STATUS "  Includes : ${MuPDF_INCLUDE_DIR}")
endif()

mark_as_advanced(MuPDF_INCLUDE_DIR MuPDF_LIBRARY MuPDF_THIRD_LIBRARY)
