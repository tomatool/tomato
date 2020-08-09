export interface IResource {
   name: string;
   type: string;
   options: object;
}

export interface IConfig {
   features_path: string[];
   resources: IResource[];
}

export interface IDictionary {
   handlers: Array<IHandler>
}

export interface IHandler {
   name: string;
   description: string;
   resources: Array<string>;
   options: Array<IOption>;
   actions: Array<any>;
}

export interface IAction {
   name: string;
   description: string;
   handle: string;
   expressions: Array<string>;
   parameters: Array<any>;
}

export interface IOption {
   name: string;
   type: string;
   description: string;
}

export interface IStep {
   resource: any;
   expression: any;
   arguments: Array<any>;
   action?: string;
}

export interface IScenario {
   title: string;
   steps: Array<IStep>
}
export interface IFeature {
   title: string;
   scenarios: Array<IScenario>;
}