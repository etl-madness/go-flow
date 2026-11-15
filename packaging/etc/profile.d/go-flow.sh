# ETL Madness Flow environment setup (/opt/flow).
# Appends /opt/flow/bin to PATH so existing system binaries are never shadowed.

case ":${PATH:-}:" in
    *:/opt/flow/bin:*) ;;
    *) export PATH="${PATH:+$PATH:}/opt/flow/bin" ;;
esac

# Expose /opt/flow/share/info to GNU info while retaining default system paths
# via the trailing colon.
case ":${INFOPATH:-}:" in
    *:/opt/flow/share/info:*) ;;
    *) export INFOPATH="${INFOPATH%:}${INFOPATH:+:}/opt/flow/share/info:" ;;
esac
