package main

// Missatges del mode -p (runHeadless). Van a part de cliMessages perquè
// afegir-los a la taula principal la realineava sencera.
var headlessMessages = map[string]cliMessage{
	"headless.step_answering":          {"step %d: answering…", "pas %d: responent…"},
	"headless.step_tools":              {"step %d: %s", "pas %d: %s"},
	"headless.fallback":                {"↪ primary failed: answered by fallback %s", "↪ primari caigut: ha respost el fallback %s"},
	"headless.cost":                    {"estimated cost: ~%s (%d↑ %d↓)", "cost estimat: ~%s (%d↑ %d↓)"},
	"headless.bad_output":              {"--output must be text or json (got %q)", "--output ha de ser text o json (rebut %q)"},
	"headless.bad_mode":                {"--mode must be code, inspect, chat, goal, autonomous or debug (got %q)", "--mode ha de ser code, inspect, chat, goal, autonomous o debug (rebut %q)"},
	"headless.bad_max_steps":           {"--max-steps must be between 1 and 200, or 0 for the configuration limit (got %d)", "--max-steps ha d'anar d'1 a 200, o 0 per al límit del config (rebut %d)"},
	"headless.autonomous_auto_approve": {"--auto-approve cannot be used with --mode autonomous: autonomous runs always need interactive approval for actions that ask, and a non-interactive run cannot give it. Use --mode code --auto-approve for unattended runs, or allow specific commands with permissions.bash_allow in the configuration.", "--auto-approve no es pot fer servir amb --mode autonomous: el mode autònom sempre demana aprovació interactiva per a les accions que pregunten, i un torn no interactiu no la pot donar. Fes servir --mode code --auto-approve per a torns desatesos, o permet ordres concretes amb permissions.bash_allow al config."},
	"headless.dir_failed":              {"--dir: %s", "--dir: %s"},
	"headless.exhausted":               {"step limit reached: the answer summarises the work done so far", "límit de passos: la resposta resumeix la feina feta fins ara"},
	"headless.verify_failed":           {"verification still failing after %d round(s): %s", "la verificació continua fallant després de %d ronda(es): %s"},
}

func init() {
	for k, v := range headlessMessages {
		cliMessages[k] = v
	}
}
