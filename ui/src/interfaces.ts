export interface ConfigResource {
    name: string;
    type: string;
    options: Record<string, string>;
}

export interface Config {
    randomize: boolean;
    stop_on_failure: boolean;
    feature_paths: string[];
    readiness_timeout: string;
    resources: ConfigResource[];
}


export interface DictionaryHandlerOption {
    name: string;
    description: string;
    type: string;
}

export interface DictionaryHandlerActionParameter {
    name: string;
    description: string;
    type: string;
}

export interface DictionaryHandlerAction {
    name: string;
    handle: string;
    description: string;
    expressions: string[];
    parameters: DictionaryHandlerActionParameter[];
    examples?: any;
}

export interface DictionaryHandler {
    name: string;
    resources: string[];
    description: string;
    options: DictionaryHandlerOption[];
    actions: DictionaryHandlerAction[];
}

export interface Dictionary {
    handlers: DictionaryHandler[];
}


