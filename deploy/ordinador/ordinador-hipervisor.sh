#!/bin/sh
# ordinador-hipervisor.sh — interfície entre l'amfitrió Gregal i el
# hipervisor per consultar, encendre, desar a disc i aturar la VM d'una
# persona (pla v2 §5, «Política d'encesa»).
#
#   ordinador-hipervisor.sh DOMINI status   → una paraula: running | saved | stopped | starting
#   ordinador-hipervisor.sh DOMINI wake     → encén o reprèn del disc (no espera que arrenqui)
#   ordinador-hipervisor.sh DOMINI suspend  → desa la memòria a disc i allibera CPU i RAM (estat normal)
#   ordinador-hipervisor.sh DOMINI stop     → apagada neta (ACPI); el disc es conserva
#
# L'amfitrió el crida amb "control": ["/usr/local/lib/gregal/ordinador-hipervisor.sh", "DOMINI"]
# al registre (GREGAL_ORDINADORS) i hi afegeix el verb com a últim argument.
# Sortida diferent de zero = error. «status» no ha de despertar mai la VM i
# té 3 s de topall a l'amfitrió; la resta, 60 s.
#
# Aquesta versió fa servir libvirt (virsh). «suspend» és `virsh managedsave`:
# la memòria va a /var/lib/libvirt/qemu/save i `virsh start` la reprèn
# (normalment uns segons; objectiu del pilot ≤ 30 s). La connexió es tria amb
# LIBVIRT_DEFAULT_URI (qemu:///system si l'amfitrió és el mateix servidor, o
# qemu+ssh://usuari@hipervisor/system). Per a un altre hipervisor, escriu un
# script amb els mateixos verbs i la mateixa sortida (README.ca.md).
set -eu

[ "$#" -eq 2 ] || { echo "ús: $0 DOMINI status|wake|suspend|stop" >&2; exit 2; }
dom=$1
verb=$2
case "$dom" in *[!A-Za-z0-9._-]*) echo "domini no vàlid" >&2; exit 2 ;; esac

state() { virsh domstate "$dom" 2>/dev/null | head -n 1 | tr -d '\r'; }
# Té una imatge de managedsave? (virsh dominfo: «Managed save: yes»)
saved_image() { virsh dominfo "$dom" 2>/dev/null | grep -qi '^Managed save: *yes'; }

case "$verb" in
status)
	s=$(state)
	case "$s" in
	running) echo running ;;
	"shut off" | shutoff)
		if saved_image; then echo saved; else echo stopped; fi
		;;
	pmsuspended) echo saved ;;
	paused | crashed | "in shutdown" | "shutting down") echo stopped ;;
	*) echo "estat desconegut: $s" >&2; exit 1 ;;
	esac
	;;
wake)
	case "$(state)" in
	running) ;;
	paused) virsh resume "$dom" >/dev/null ;;
	pmsuspended) virsh dompmwakeup "$dom" >/dev/null ;;
	*) virsh start "$dom" >/dev/null ;; # reprèn la imatge desada si n'hi ha
	esac
	;;
suspend)
	case "$(state)" in
	running | paused) virsh managedsave "$dom" --running >/dev/null ;;
	*) ;; # ja és desada o aturada
	esac
	;;
stop)
	case "$(state)" in
	running) virsh shutdown "$dom" >/dev/null ;;
	*) ;;
	esac
	;;
*)
	echo "verb desconegut: $verb" >&2
	exit 2
	;;
esac
