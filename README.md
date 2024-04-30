## Checkpoint

Para cada checkpoint de control obligatorio, se debe crear un tag en el
repositorio con el siguiente formato:

```
checkpoint-{número}
```

Donde `{número}` es el número del checkpoint.

Para crear un tag y subirlo al repositorio, podemos utilizar los siguientes
comandos:

```bash
git tag -a checkpoint-{número} -m "Checkpoint {número}"
git push origin checkpoint-{número}
```

Asegúrense de que el código compila y cumple con los requisitos del checkpoint
antes de subir el tag.

### Checkpoint 1

- [x] Familiarizarse con Linux y su consola, el entorno de desarrollo y el repositorio.
- [x] Aprender a utilizar las Commons, principalmente las funciones para listas, archivos de configuración y logs.
- [x] Definir el Protocolo de Comunicación.
- [x] Todas las API del módulo Kernel definidas por la cátedra están creadas y retornan datos hardcodeados.
- [x] Todos los módulos están creados y son capaces de inicializarse con al menos una API.

### Checkpoint 2

- Módulo Kernel:
  
    - [ ] Es capaz de crear un PCB y planificarlo por FIFO y RR.
    - [ ] Es capaz de enviar un proceso a la CPU para que sea procesado.

- Módulo CPU:
  
    - [ ] Se conecta a Kernel y recibe un PCB.
    - [ ] Es capaz de conectarse a la memoria y solicitar las instrucciones.
    - [ ] Es capaz de ejecutar un ciclo básico de instrucción.
    - [ ] Es capaz de resolver las operaciones: SET, SUM, SUB, JNZ e IO_GEN_SLEEP.

- Módulo Memoria:
  
    - [ ] Se encuentra creado y acepta las conexiones.
    - [ ] Es capaz de abrir los archivos de pseudocódigo y envía las instrucciones al CPU.

- Módulo Interfaz I/O:
- 
    - [x] Se encuentra desarrollada la Interfaz Genérica.


### Checkpoint 3

- Módulo Kernel:

    - [ ] Es capaz de planificar por VRR.
    - [ ] Es capaz de realizar manejo de recursos.
    - [ ] Es capaz de manejar el planificador de largo plazo

- Módulo CPU:

    - [ ] Es capaz de resolver las operaciones: MOV_IN, MOV_OUT, RESIZE, COPY_STRING, IO_STDIN_READ, IO_STDOUT_WRITE.

- Módulo Memoria:

    - [ ] Se encuentra completamente desarrollada.

- Módulo Interfaz I/O:

    - [ ] Se encuentran desarrolladas las interfaces STDIN y STDOUT.

## Entregas finales

- [ ] Finalizar el desarrollo de todos los procesos.
- [ ] Probar de manera intensiva el TP en un entorno distribuido.
- [ ] Todos los componentes del TP ejecutan los requerimientos de forma integral.