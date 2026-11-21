# ETL Madness Flow environment setup (/opt/flow).
# Appends /opt/flow/bin to PATH so existing system binaries are never shadowed.
# Safe under POSIX sh, ksh, bash, dash, and zsh, including 'set -u' (nounset).

case ":${PATH:-}:" in
    *:/opt/flow/bin:*) ;;
    *)
        if [ -n "${PATH:-}" ]; then
            export PATH="${PATH}:/opt/flow/bin"
        else
            export PATH="/opt/flow/bin"
        fi
        ;;
esac

# Expose /opt/flow/share/info to GNU info while retaining default system paths
# via the trailing colon.
case ":${INFOPATH:-}:" in
    *:/opt/flow/share/info:*) ;;
    *)
        _goflow_infopath="${INFOPATH:-}"
        _goflow_infopath="${_goflow_infopath%:}"
        if [ -n "$_goflow_infopath" ]; then
            export INFOPATH="${_goflow_infopath}:/opt/flow/share/info:"
        else
            export INFOPATH="/opt/flow/share/info:"
        fi
        unset _goflow_infopath
        ;;
esac

# On OpenBSD, man(1) does not auto-derive ../share/man from PATH; a leading
# colon in MANPATH appends to the default /etc/man.conf search path.
if [ "$(uname -s 2>/dev/null || true)" = "OpenBSD" ]; then
    case ":${MANPATH:-}:" in
        *:/opt/flow/share/man:*) ;;
        *)
            if [ -n "${MANPATH:-}" ]; then
                export MANPATH="${MANPATH}:/opt/flow/share/man"
            else
                export MANPATH=":/opt/flow/share/man"
            fi
            ;;
    esac
fi
