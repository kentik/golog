golog
=====

A high performance wrapper around Syslog.

Usage:

    var (
      logName
      logLevel
      logAddress
      logNetwork
    )

	if err := logger.SetLogName(logName); err != nil {
		fatal(nil, "Cannot set log name for program")
	}

	// And set the logger to write to a custom socket.
	if logAddress != "" && logNetwork != "" {
		if err := logger.SetCustomSocket(logAddress, logNetwork); err != nil {
			fatal(nil, "Cannot set custom log socket program: %s %s %v", logAddress, logNetwork, err)
		}
	}

	if ll, ok := logger.CfgLevels[strings.ToLower(logLevel)]; !ok {
		fatal(nil, "Unsupported log level: "+logLevel)
	} else {
		if log := logger.New(ll); log == nil {
			fatal(nil, "Cannot start logger")
		}
	}
