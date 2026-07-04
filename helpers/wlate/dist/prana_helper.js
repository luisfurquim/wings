/**
 * prana_helper.js
 *
 * Helper JS mínimo para interop com o wings Go/WASM.
 * Deve ser carregado ANTES do wasm_exec.js e do binário .wasm.
 *
 * Define window._pranaDef(tagName, ctor, connected, attrChanged, disconnected, observed,
 * formAssociated, formReset, formDisabled) que é chamado pelo Go via
 * DefineAll() / defineCustomElement().
 */

(function() {
   "use strict";

   /**
    * _pranaDef registra um custom element cujos callbacks de ciclo de vida
    * são implementados em Go/WASM.
    *
    * @param {string}   tagName      - nome do custom element (e.g. "my-widget")
    * @param {Function} ctor         - callback Go para constructor(self)
    * @param {Function} connected    - callback Go para connectedCallback(self)
    * @param {Function} attrChanged  - callback Go para attributeChangedCallback(self,name,old,new)
    * @param {Function} disconnected - callback Go para disconnectedCallback(self)
    * @param {string[]} observed     - lista de atributos observados
    * @param {boolean}  formAssociated - true → elemento form-associated: a classe
    *                   declara `static formAssociated` e o constructor chama
    *                   attachInternals(), exposto ao Go como `_internals`
    *                   (setValidity/setFormValue participam do <form> nativo)
    * @param {Function} formReset    - callback Go para formResetCallback(self);
    *                   invocado pelo browser em form.reset() (só p/ form-associated)
    * @param {Function} formDisabled - callback Go para formDisabledCallback(self,disabled);
    *                   invocado quando um <fieldset disabled> ancestral alterna
    */
   window._pranaDef = function(tagName, ctor, connected, attrChanged, disconnected, observed, formAssociated, formReset, formDisabled) {
      customElements.define(
         tagName,
         class extends HTMLElement {
            static get formAssociated() {
               return !!formAssociated;
            }

            static get observedAttributes() {
               return observed || [];
            }

            constructor() {
               super();
               if (formAssociated) {
                  this._internals = this.attachInternals();
               }
               // Encaminha para o Go
               ctor(this);
            }

            connectedCallback() {
               connected(this);
            }

            attributeChangedCallback(name, oldValue, newValue) {
               attrChanged(this, name, oldValue ?? "", newValue ?? "");
            }

            disconnectedCallback() {
               disconnected(this);
            }

            // Form lifecycle — only fired by the browser for form-associated
            // elements, so no formAssociated guard is needed here.
            formResetCallback() {
               formReset(this);
            }

            formDisabledCallback(disabled) {
               formDisabled(this, disabled);
            }
         }
      );
   };
})();
