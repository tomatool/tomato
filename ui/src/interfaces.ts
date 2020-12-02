export interface Resource {
    name: string;
    type: string;
    options: Record<string, string>;
}

export interface Config {
    randomize: boolean;
    stop_on_failure: boolean;
    feature_paths: string[];
    readiness_timeout: string;
    resources: Resource[];
}

