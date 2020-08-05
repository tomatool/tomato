import { IDictionary, IOption } from './interfaces';

export function getListOfResources(dictionary: IDictionary): Array<string> {
   return dictionary.handlers.map((handler) => {
        return handler.resources.map((resource) => {
            return resource;
        });
   }).flat();
}

export function getResourceOptions(dictionary: IDictionary, resourceType: string): Array<IOption> {
   const handler = dictionary.handlers.find(handler => {
        const selectedHandler = handler.resources.find((resource) => resource === resourceType);
         
        if (selectedHandler) return true;

        return false;
   });

   if (handler === undefined) return [];

   return handler.options;
}
