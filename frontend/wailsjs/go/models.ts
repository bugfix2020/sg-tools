export namespace app {
	
	export interface WindowInfo {
	    title: string;
	    processName: string;
	    pid: number;
	}
	export interface ProfileView {
	    id: string;
	    name: string;
	    bindings: clicker.KeyBinding[];
	    target?: WindowInfo;
	    state: string;
	    lastError?: string;
	}
	export interface Snapshot {
	    profiles: ProfileView[];
	    activeProfileId: string;
	    hotkeys: clicker.HotkeyConfig;
	}

}

export namespace clicker {
	
	export interface BatchSkip {
	    profileId: string;
	    reason: string;
	}
	export interface BatchResult {
	    started: string[];
	    skipped: BatchSkip[];
	}
	
	export interface HotkeyConfig {
	    activeStart: string;
	    activeStop: string;
	    globalStart: string;
	    globalStop: string;
	}
	export interface KeyBinding {
	    code: string;
	    label: string;
	    delayMs: number;
	}

}

