cmake_minimum_required(VERSION 3.22)
project(pdfengine VERSION 1.0.0 LANGUAGES CXX C)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)  # required: Go links PIC objects

if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

# ── Output: static library ────────────────────────────────────────────
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/lib)

# ── Options ───────────────────────────────────────────────────────────
option(ENGINE_USE_SYSTEM_MUPDF  "Use system MuPDF"  OFF)
option(ENGINE_USE_SYSTEM_QPDF   "Use system QPDF"   OFF)
option(ENGINE_ENABLE_LTO        "Enable LTO"         ON)

# ── Platform ──────────────────────────────────────────────────────────
if(WIN32)
    add_compile_definitions(PM_WINDOWS=1 NOMINMAX WIN32_LEAN_AND_MEAN)
else()
    add_compile_definitions(PM_LINUX=1)
endif()

# ── Compiler flags ────────────────────────────────────────────────────
if(MSVC)
    add_compile_options(/W4 /MP /utf-8 /EHsc)
else()
    add_compile_options(-Wall -Wextra -Wno-unused-parameter)
    add_compile_options($<$<CONFIG:Release>:-O3>)
    add_compile_options($<$<CONFIG:Release>:-march=native>)
    add_compile_options($<$<CONFIG:Release>:-ffunction-sections>)
    add_compile_options($<$<CONFIG:Release>:-fdata-sections>)
    # Suppress QPDF PointerHolder deprecation warning
    add_compile_definitions(POINTERHOLDER_TRANSITION=4)
endif()

if(ENGINE_ENABLE_LTO AND CMAKE_BUILD_TYPE STREQUAL "Release")
    include(CheckIPOSupported)
    check_ipo_supported(RESULT _ipo OUTPUT _msg)
    if(_ipo)
        set(CMAKE_INTERPROCEDURAL_OPTIMIZATION TRUE)
        message(STATUS "LTO enabled")
    endif()
endif()

# ── Find MuPDF ────────────────────────────────────────────────────────
list(APPEND CMAKE_MODULE_PATH "${CMAKE_SOURCE_DIR}/../../cmake")
if(ENGINE_USE_SYSTEM_MUPDF)
    find_package(PkgConfig REQUIRED)
    pkg_check_modules(MUPDF mupdf)
    if(MUPDF_FOUND)
        add_compile_definitions(PDFMASTER_MUPDF_AVAILABLE=1)
        message(STATUS "MuPDF: found via pkg-config")
    else()
        message(WARNING "MuPDF not found — rendering will use stub")
    endif()
else()
    find_package(MuPDF QUIET)
    if(MuPDF_FOUND)
        add_compile_definitions(PDFMASTER_MUPDF_AVAILABLE=1)
        message(STATUS "MuPDF: found via FindMuPDF")
    endif()
endif()

# ── Find QPDF ─────────────────────────────────────────────────────────
if(ENGINE_USE_SYSTEM_QPDF)
    find_package(PkgConfig REQUIRED)
    pkg_check_modules(QPDF libqpdf)
    if(QPDF_FOUND)
        add_compile_definitions(PDFMASTER_QPDF_AVAILABLE=1)
        message(STATUS "QPDF: found via pkg-config")
    else()
        message(WARNING "QPDF not found — merge/compress/split will use stubs")
    endif()
else()
    find_package(QPDF QUIET)
    if(QPDF_FOUND)
        add_compile_definitions(PDFMASTER_QPDF_AVAILABLE=1)
        message(STATUS "QPDF: found via FindQPDF")
    endif()
endif()

find_package(ZLIB REQUIRED)

# ── Source files ──────────────────────────────────────────────────────
set(ENGINE_SOURCES
    src/core/PdfDocument.cpp
    src/core/PdfMerger.cpp
    src/core/PdfCompressor.cpp
    src/core/PdfSplitter.cpp
    src/bridge/pdfengine_c.cpp
)

# ── Static library target ─────────────────────────────────────────────
add_library(pdfengine STATIC ${ENGINE_SOURCES})

target_include_directories(pdfengine
    PUBLIC  ${CMAKE_SOURCE_DIR}/include
    PRIVATE ${CMAKE_SOURCE_DIR}/src/core
            ${CMAKE_SOURCE_DIR}/src/bridge
)

# Link MuPDF
if(MUPDF_FOUND)
    if(ENGINE_USE_SYSTEM_MUPDF)
        target_include_directories(pdfengine PRIVATE ${MUPDF_INCLUDE_DIRS})
        target_link_libraries(pdfengine PRIVATE ${MUPDF_LIBRARIES})
    else()
        target_include_directories(pdfengine PRIVATE ${MuPDF_INCLUDE_DIRS})
        target_link_libraries(pdfengine PRIVATE ${MuPDF_LIBRARIES})
    endif()
endif()

# Link QPDF
if(QPDF_FOUND)
    if(ENGINE_USE_SYSTEM_QPDF)
        target_include_directories(pdfengine PRIVATE ${QPDF_INCLUDE_DIRS})
        target_link_libraries(pdfengine PRIVATE ${QPDF_LIBRARIES})
    else()
        target_include_directories(pdfengine PRIVATE ${QPDF_INCLUDE_DIRS})
        target_link_libraries(pdfengine PRIVATE ${QPDF_LIBRARIES})
    endif()
endif()

target_link_libraries(pdfengine PRIVATE ZLIB::ZLIB)

# On Linux: also need stdc++ and pthread when Go links the .a
if(UNIX)
    target_link_libraries(pdfengine PRIVATE stdc++ m pthread dl)
endif()

# ── Install headers + library ─────────────────────────────────────────
install(FILES include/pdfengine.h DESTINATION include)
install(TARGETS pdfengine ARCHIVE DESTINATION lib)

# ── Summary ───────────────────────────────────────────────────────────
message(STATUS "──────────────────────────────────────")
message(STATUS " pdfengine static library")
message(STATUS " MuPDF : ${MUPDF_FOUND}")
message(STATUS " QPDF  : ${QPDF_FOUND}")
message(STATUS " Build : ${CMAKE_BUILD_TYPE}")
message(STATUS "──────────────────────────────────────")
