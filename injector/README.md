# JSON/YAML 数据注入工具

一个用 Go 编写的命令行工具，可以在 JSON 和 YAML 文件中插入、更新和删除数据。

## 功能特性

- 支持 JSON 和 YAML 文件格式
- 支持三种操作：set（设置/替换）、insert（仅插入）、delete（删除）
- 支持点号路径（如 `.name`）和中括号数组索引（如 `users[0].id`）
- 自动识别输入文件格式
- 支持输出为 JSON 或 YAML 格式
- 支持输出到文件或标准输出

## 安装

```bash
go mod tidy
go build -o injector main.go
```

## 使用方法

### 基本语法

```bash
./injector -f <文件> [选项]
```

### 操作类型

1. **--set path=value**: 设置值（如果路径存在则替换，不存在则创建）
2. **--insert path=value**: 插入值（仅当路径不存在时）
3. **--delete path**: 删除指定路径的值

### 路径格式

- 简单字段：`name`、`age`
- 嵌套字段：`users.0.name`、`config.debug`
- 数组索引：`users.0`、`items.1.id`
- 数组插入符号：
  - `^`：在数组开头插入（如 `users^.name`）
  - `$`：在数组末尾插入（如 `users$.name`）

### 示例

#### 1. 基本设置操作

```bash
# 设置简单字段
./injector -f data.yaml --set name=张三

# 设置嵌套字段
./injector -f data.yaml --set "users.0.name=李四"

# 设置数组元素
./injector -f data.yaml --set "users.0.profile.age=25"

# 在数组开头插入元素
./injector -f data.yaml --set "users^.name=新用户"

# 在数组末尾插入元素
./injector -f data.yaml --set "users$.name=新用户"
```

#### 2. 插入操作（仅当字段不存在时）

```bash
# 插入新字段
./injector -f data.yaml --insert newField=新值

# 插入嵌套字段
./injector -f data.yaml --insert "users.0.newField=新值"

# 在数组开头插入新元素
./injector -f data.yaml --insert "users^.name=新用户"

# 在数组末尾插入新元素
./injector -f data.yaml --insert "users$.name=新用户"
```

#### 3. 删除操作

```bash
# 删除字段
./injector -f data.yaml --delete age

# 删除嵌套字段
./injector -f data.yaml --delete "users.0.profile.city"
```

#### 4. 组合操作

```bash
# 同时进行多个操作
./injector -f data.yaml \
  --set name=小李 \
  --set "users.0.profile.age=35" \
  --insert newField=新值 \
  --delete oldField
```

#### 5. 输出格式控制

```bash
# 输出为 JSON 格式
./injector -f data.yaml --set name=张三 -o json

# 输出到文件
./injector -f data.yaml --set name=张三 -out output.yaml

# 自动保存到原文件
./injector -f data.yaml --set name=张三 -o save
```

### 选项说明

- `-f <文件>`: 指定输入文件（JSON 或 YAML）
- `-o <格式>`: 指定输出格式（yaml、json 或 save，默认为 yaml）
- `-out <文件>`: 指定输出文件（默认为标准输出）
- `--set path=value`: 设置值
- `--insert path=value`: 插入值
- `--delete path`: 删除值

### 注意事项

1. **路径格式**: 使用点号表示对象字段，中括号表示数组索引
2. **值格式**: 字符串会自动加引号，数字和布尔值保持原样
3. **JSON 值**: 可以使用 JSON 格式的值，如 `{"key":"value"}`
4. **数组索引**: 从 0 开始计数
5. **文件格式**: 根据文件扩展名自动识别格式（.json、.yaml、.yml）
6. **自动保存**: 使用 `-o save` 可以直接修改原文件
7. **数组插入**: 使用 `^` 在数组开头插入，`$` 在数组末尾插入

### 示例文件

#### input.yaml
```yaml
name: 张三
age: 25
users:
  - id: 1
    name: 李四
    profile:
      age: 30
      city: 北京
  - id: 2
    name: 王五
    profile:
      age: 28
      city: 上海
config:
  debug: true
  port: 8080
```

#### 操作示例
```bash
# 修改用户名
./injector -f input.yaml --set name=小李

# 修改第一个用户的年龄
./injector -f input.yaml --set "users.0.profile.age=35"

# 在数组开头插入新用户
./injector -f input.yaml --set "users^.name=新用户"

# 在数组末尾插入新用户
./injector -f input.yaml --set "users$.name=新用户"

# 添加新字段
./injector -f input.yaml --insert newField=新值

# 删除年龄字段
./injector -f input.yaml --delete age
```

## 依赖

- `github.com/tidwall/sjson`: JSON 操作库
- `gopkg.in/yaml.v3`: YAML 解析库 