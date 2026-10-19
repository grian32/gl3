# Sourced by CI after setup-llvm. Statically links LLVM (byollvm + --link-static)
# so gl3 only needs a C compiler on the user's machine.
case "$RUNNER_OS" in
Windows) llvm_config="/ucrt64/opt/llvm$LLVM_VERSION/bin/llvm-config" ;;
macOS) llvm_config="$(brew --prefix "llvm@$LLVM_VERSION")/bin/llvm-config" ;;
*) llvm_config="/usr/lib/llvm-$LLVM_VERSION/bin/llvm-config" ;;
esac
export PATH="$PATH:$(dirname "$llvm_config")"

sys="$("$llvm_config" --link-static --system-libs)"
case "$RUNNER_OS" in
Windows)
	# MSYS2's llvm-config omits libffi and zstd; -static drops the MSYS2 DLLs.
	sys="$sys -lffi -lzstd -static"
	;;
macOS)
	# Homebrew's zstd isn't on users' machines; libffi, libxml2 and libz ship with macOS.
	sys="$(echo "$sys" | sed "s#-lzstd#$(brew --prefix zstd)/lib/libzstd.a#") -lffi"
	;;
*)
	# Archive-link the libs that aren't on every distro; libxml2 is only for unused WindowsManifest.
	sys="$(echo "$sys" | sed -e 's/-lzstd/-l:libzstd.a/' -e 's/-lxml2//') -l:libffi.a -static-libstdc++ -static-libgcc"
	;;
esac

CGO_CPPFLAGS="$("$llvm_config" --cppflags)"
CGO_CXXFLAGS="$("$llvm_config" --cxxflags)"
CGO_LDFLAGS="$("$llvm_config" --link-static --ldflags --libs) $sys"
# byollvm drops go-llvm's cgo flags; llvmNN still selects the matching API.
GL3_BUILD_TAGS="byollvm,llvm$LLVM_VERSION"
export CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS GL3_BUILD_TAGS

go version
"$llvm_config" --version
